package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxPushRetries = 6

type Options struct {
	Repo       string
	MaxFiles   int
	MaxSize    Size
	Message    string
	MessageSet bool
	Push       bool
	Remote     string
	RemoteSet  bool
	Branch     string
	BranchSet  bool
	Retries    int
	LogFile    string
	DryRun     bool
	JSON       bool
	Quiet      bool
	SpacingMin time.Duration
	SpacingMax time.Duration
	SpacingSet bool
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string { return e.Message }

func usageError(format string, args ...any) error {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

func validateOptions(opts Options) error {
	if strings.TrimSpace(opts.Repo) == "" {
		return usageError("--repo must not be empty")
	}
	if opts.MaxFiles < 0 {
		return usageError("--max-files must not be negative")
	}
	if opts.MaxFiles == 0 && opts.MaxSize == 0 {
		return usageError("specify at least one criterion: --max-files and/or --max-size (see --help)")
	}
	if opts.MaxSize < 0 {
		return usageError("--max-size must not be negative")
	}
	if opts.Retries < 0 {
		return usageError("--retries must not be negative")
	}
	if opts.Retries > maxPushRetries {
		return usageError("--retries must not exceed %d", maxPushRetries)
	}
	if opts.SpacingSet && (opts.SpacingMin <= 0 || opts.SpacingMax <= 0 || opts.SpacingMin > opts.SpacingMax) {
		return usageError("--spacing must be a positive duration or range with min <= max")
	}
	if opts.SpacingSet && !opts.Push && !opts.DryRun {
		return usageError("--spacing requires --push")
	}
	if opts.JSON && !opts.DryRun {
		return usageError("--json requires --dry-run")
	}
	if strings.TrimSpace(opts.Message) == "" && (!opts.DryRun || opts.MessageSet) {
		return usageError("--message must not be empty")
	}
	if strings.TrimSpace(opts.Remote) == "" && (opts.Push || opts.RemoteSet) {
		return usageError("--remote must not be empty")
	}
	if opts.BranchSet && strings.TrimSpace(opts.Branch) == "" {
		return usageError("--branch must not be empty")
	}
	return nil
}

type planChunk struct {
	Index                     int    `json:"index"`
	Files                     []File `json:"files"`
	Size                      int64  `json:"size"`
	UncompressedBytesEstimate int64  `json:"uncompressed_bytes_estimate"`
	ProposedMessage           string `json:"proposed_message"`
}

type runDeps struct {
	rand      *rand.Rand
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error
	stderrTTY bool
}

func defaultRunDeps(stderr io.Writer) runDeps {
	return runDeps{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		now:  time.Now,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		stderrTTY: isTTY(stderr),
	}
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func Run(opts Options, stdout, stderr io.Writer) error {
	return run(context.Background(), opts, stdout, stderr, defaultRunDeps(stderr))
}

func RunContext(ctx context.Context, opts Options, stdout, stderr io.Writer) error {
	return run(ctx, opts, stdout, stderr, defaultRunDeps(stderr))
}

func run(ctx context.Context, opts Options, stdout, stderr io.Writer, deps runDeps) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	logger, err := NewLogger(stderr, opts.LogFile, opts.Quiet)
	if err != nil {
		return err
	}
	defer logger.Close()
	// Finish any in-place ETA before Main writes a returned error to stderr.
	defer logger.clearETA()

	repo, err := filepath.Abs(opts.Repo)
	if err != nil {
		return err
	}
	if _, err := git(repo, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository: %s", repo)
	}

	files, err := pendingFiles(repo)
	if err != nil {
		return err
	}
	if opts.DryRun {
		chunks := chunkFiles(files, opts.MaxFiles, int64(opts.MaxSize))
		staged, err := hasStagedChanges(repo)
		if err != nil {
			return err
		}
		if staged {
			logger.Warn("note: the index has staged changes; a real run would refuse")
		}
		message := opts.Message
		if strings.TrimSpace(message) == "" {
			message = defaultMessage
		}
		return printPlan(chunks, message, opts.JSON, opts.SpacingMin, opts.SpacingMax, opts.SpacingSet, stdout)
	}
	branch := opts.Branch
	if opts.Push && branch == "" {
		branch, err = currentBranch(repo)
		if err != nil {
			return usageError("--push requires an attached HEAD unless --branch is set")
		}
	}
	if len(files) > 0 {
		staged, err := hasStagedChanges(repo)
		if err != nil {
			return err
		}
		if staged {
			return fmt.Errorf("the Git index contains staged changes; unstage them before running git-chunks")
		}
	}

	pushed := false
	if opts.Push {
		unpushed, err := hasUnpushedCommits(repo, opts.Remote, branch)
		if err != nil {
			return fmt.Errorf("could not check for existing unpushed commits; no new commits were created: %w", err)
		}
		if unpushed {
			logger.Progress("Pushing existing unpushed commits before creating chunks...")
			if err := pushWithRetry(ctx, repo, opts.Remote, branch, opts.Retries, logger, deps.sleep); err != nil {
				return fmt.Errorf("could not push existing commits; no new commits were created: %w", err)
			}
			pushed = true
		}
	}
	if len(files) == 0 {
		logger.Progress("Nothing to commit.")
		return nil
	}

	chunks := chunkFiles(files, opts.MaxFiles, int64(opts.MaxSize))
	total := len(chunks)
	eta := etaEstimate{}
	logger.Progress("%d file(s) -> %d chunk(s)", len(files), total)

	for i, chunk := range chunks {
		label := fmt.Sprintf("[%d/%d] %d file(s), %s", i+1, total, len(chunk), formatSize(chunkSize(chunk)))

		addArgs := []string{"add", "-A", "--"}
		for _, f := range chunk {
			addArgs = append(addArgs, f.Path)
		}
		if _, err := git(repo, addArgs...); err != nil {
			return restoreIndexAfterFailure(repo, err)
		}
		message := chunkMessage(opts.Message, i+1, total)
		if _, err := git(repo, "commit", "-m", message); err != nil {
			return restoreIndexAfterFailure(repo, err)
		}
		logger.Progress("%s committed", label)

		if opts.Push {
			logger.ETA(eta.message(deps.now(), chunks[i:], opts.SpacingMin, opts.SpacingMax, opts.SpacingSet), deps.stderrTTY)
			if pushed && opts.SpacingSet {
				if err := deps.sleep(ctx, randomSpacing(deps.rand, opts.SpacingMin, opts.SpacingMax)); err != nil {
					return fmt.Errorf("spacing interrupted; committed work is safe; rerun the same command to resume: %w", err)
				}
			}
			started := deps.now()
			if err := pushWithRetry(ctx, repo, opts.Remote, branch, opts.Retries, logger, deps.sleep); err != nil {
				return fmt.Errorf("%w\nCommitted work is safe; rerun the same command to resume", err)
			}
			pushed = true
			logger.Progress("    pushed to %s/%s", opts.Remote, branch)
			eta.add(deps.now().Sub(started), chunkSize(chunk))
		}
	}

	logger.Progress("Done: %d chunk(s) processed.", total)
	return nil
}

func chunkMessage(message string, index, total int) string {
	return fmt.Sprintf("%s (%d/%d)", message, index, total)
}

func restoreIndexAfterFailure(repo string, cause error) error {
	args := []string{"reset", "--mixed", "--quiet", "HEAD"}
	if !gitSuccess(repo, "rev-parse", "--verify", "HEAD") {
		args = []string{"read-tree", "--empty"}
	}
	if _, err := git(repo, args...); err != nil {
		return fmt.Errorf("%w; the commit failed and the Git index could not be restored: %v", cause, err)
	}
	return fmt.Errorf("%w; the Git index was restored and working-tree changes were preserved", cause)
}

func printPlan(chunks [][]File, message string, asJSON bool, spacingMin, spacingMax time.Duration, spacingSet bool, out io.Writer) error {
	if asJSON {
		plan := make([]planChunk, len(chunks))
		for i, chunk := range chunks {
			size := chunkSize(chunk)
			plan[i] = planChunk{
				Index:                     i + 1,
				Files:                     chunk,
				Size:                      size,
				UncompressedBytesEstimate: size,
				ProposedMessage:           chunkMessage(message, i+1, len(chunks)),
			}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	for i, chunk := range chunks {
		if _, err := fmt.Fprintf(out, "[%d/%d] %d file(s), %s (pre-pack estimate)\n", i+1, len(chunks), len(chunk), formatSize(chunkSize(chunk))); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "    proposed message: %s\n", chunkMessage(message, i+1, len(chunks))); err != nil {
			return err
		}
		for _, f := range chunk {
			if _, err := fmt.Fprintf(out, "    %s (%s)\n", f.Path, formatSize(f.Size)); err != nil {
				return err
			}
		}
	}
	if len(chunks) > 0 {
		if _, err := fmt.Fprintln(out, "estimates are pre-pack; actual sizes will differ"); err != nil {
			return err
		}
	}
	if spacingSet && len(chunks) > 1 {
		if _, err := fmt.Fprintf(out, "expected total spacing delay: %s-%s\n", spacingMin*time.Duration(len(chunks)-1), spacingMax*time.Duration(len(chunks)-1)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "Dry run: no commits made.")
	return err
}

func pushWithRetry(ctx context.Context, repo, remote, branch string, retries int, logger *Logger, sleep func(context.Context, time.Duration) error) error {
	var err error
	for attempt := 0; ; attempt++ {
		if _, err = git(repo, "push", remote, "HEAD:refs/heads/"+branch); err == nil {
			return nil
		}
		if attempt >= retries {
			return fmt.Errorf("push failed after %d attempt(s): %w", attempt+1, err)
		}
		if !isTransientPushError(err) {
			return fmt.Errorf("push failed: %w", err)
		}
		delay := time.Duration(1<<attempt) * time.Second
		logger.Error("push failed (attempt %d/%d), retrying in %s: %v", attempt+1, retries+1, delay, err)
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}
}

func parseSpacing(value string) (time.Duration, time.Duration, error) {
	for i := 1; i < len(value); i++ {
		if value[i] == '-' {
			min, err := time.ParseDuration(value[:i])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid spacing %q: %w", value, err)
			}
			max, err := time.ParseDuration(value[i+1:])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid spacing %q: %w", value, err)
			}
			if min <= 0 || max <= 0 || min > max {
				return 0, 0, fmt.Errorf("spacing must be a positive duration or range with min <= max")
			}
			return min, max, nil
		}
	}
	min, err := time.ParseDuration(value)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid spacing %q: %w", value, err)
	}
	if min <= 0 {
		return 0, 0, fmt.Errorf("spacing must be a positive duration or range with min <= max")
	}
	return min, min, nil
}

func randomSpacing(r *rand.Rand, min, max time.Duration) time.Duration {
	if min == max {
		return min
	}
	return min + time.Duration(r.Int63n(int64(max-min)+1))
}

type etaEstimate struct {
	rate   float64
	weight float64
}

func (e *etaEstimate) add(duration time.Duration, bytes int64) {
	if bytes <= 0 {
		bytes = 1
	}
	weight := float64(bytes)
	sample := float64(duration) / weight
	if e.weight == 0 {
		e.rate, e.weight = sample, weight
		return
	}
	e.rate = (e.rate*e.weight*0.5 + sample*weight) / (e.weight*0.5 + weight)
	e.weight = e.weight*0.5 + weight
}

func (e etaEstimate) message(now time.Time, remaining [][]File, min, max time.Duration, spacing bool) string {
	if e.weight == 0 {
		return "ETA pending"
	}
	var bytes int64
	for _, chunk := range remaining {
		bytes += chunkSize(chunk)
	}
	eta := time.Duration(e.rate * float64(bytes))
	if spacing && len(remaining) > 0 {
		eta += (min + max) / 2 * time.Duration(len(remaining))
	}
	return fmt.Sprintf("ETA %s (completes ~%s)", eta.Round(time.Second), now.Add(eta).Format("15:04"))
}

func isTransientPushError(err error) bool {
	text := strings.ToLower(err.Error())
	for _, phrase := range []string{"timed out", "connection reset", "could not resolve host", "failed to connect", "remote end hung up", "unexpected disconnect", "temporarily unavailable", "http 5"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
