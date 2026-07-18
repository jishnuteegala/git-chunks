package cli

import (
	"encoding/json"
	"fmt"
	"io"
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
	Index int    `json:"index"`
	Files []File `json:"files"`
	Size  int64  `json:"size"`
}

func Run(opts Options, stdout, stderr io.Writer) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	logger, err := NewLogger(stderr, opts.LogFile, opts.Quiet)
	if err != nil {
		return err
	}
	defer logger.Close()

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
		return printPlan(chunks, opts.JSON, stdout)
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

	if opts.Push {
		unpushed, err := hasUnpushedCommits(repo, opts.Remote, branch)
		if err != nil {
			return fmt.Errorf("could not check for existing unpushed commits; no new commits were created: %w", err)
		}
		if unpushed {
			logger.Progress("Pushing existing unpushed commits before creating chunks...")
			if err := pushWithRetry(repo, opts.Remote, branch, opts.Retries, logger); err != nil {
				return fmt.Errorf("could not push existing commits; no new commits were created: %w", err)
			}
		}
	}
	if len(files) == 0 {
		logger.Progress("Nothing to commit.")
		return nil
	}

	chunks := chunkFiles(files, opts.MaxFiles, int64(opts.MaxSize))
	total := len(chunks)
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
		message := fmt.Sprintf("%s (%d/%d)", opts.Message, i+1, total)
		if _, err := git(repo, "commit", "-m", message); err != nil {
			return restoreIndexAfterFailure(repo, err)
		}
		logger.Progress("%s committed", label)

		if opts.Push {
			if err := pushWithRetry(repo, opts.Remote, branch, opts.Retries, logger); err != nil {
				return fmt.Errorf("%w\nCommitted work is safe; rerun the same command to resume", err)
			}
			logger.Progress("    pushed to %s/%s", opts.Remote, branch)
		}
	}

	logger.Progress("Done: %d chunk(s) processed.", total)
	return nil
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

func printPlan(chunks [][]File, asJSON bool, out io.Writer) error {
	if asJSON {
		plan := make([]planChunk, len(chunks))
		for i, chunk := range chunks {
			plan[i] = planChunk{Index: i + 1, Files: chunk, Size: chunkSize(chunk)}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	for i, chunk := range chunks {
		fmt.Fprintf(out, "[%d/%d] %d file(s), %s\n", i+1, len(chunks), len(chunk), formatSize(chunkSize(chunk)))
		for _, f := range chunk {
			fmt.Fprintf(out, "    %s (%s)\n", f.Path, formatSize(f.Size))
		}
	}
	fmt.Fprintln(out, "Dry run: no commits made.")
	return nil
}

func pushWithRetry(repo, remote, branch string, retries int, logger *Logger) error {
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
		time.Sleep(delay)
	}
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
