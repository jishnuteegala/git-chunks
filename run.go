package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Repo     string
	MaxFiles int
	MaxSize  Size
	Message  string
	Push     bool
	Remote   string
	Branch   string
	Retries  int
	LogFile  string
	DryRun   bool
	JSON     bool
	Quiet    bool
}

type planChunk struct {
	Index int    `json:"index"`
	Files []File `json:"files"`
	Size  int64  `json:"size"`
}

func Run(opts Options, stdout, stderr io.Writer) error {
	if opts.MaxFiles <= 0 && opts.MaxSize <= 0 {
		return errors.New("specify at least one criterion: --max-files and/or --max-size (see --help)")
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
	if len(files) == 0 {
		logger.Progress("Nothing to commit.")
		return nil
	}

	chunks := chunkFiles(files, opts.MaxFiles, int64(opts.MaxSize))
	total := len(chunks)

	if opts.DryRun {
		return printPlan(chunks, opts.JSON, stdout)
	}

	logger.Progress("%d file(s) -> %d chunk(s)", len(files), total)

	branch := opts.Branch
	if opts.Push {
		if branch == "" {
			if branch, err = currentBranch(repo); err != nil {
				return err
			}
		}
		if ahead := unpushedCommits(repo, opts.Remote, branch); ahead > 0 {
			logger.Progress("Resuming: %d unpushed commit(s) from a previous run will ride along with the first push.", ahead)
		}
	}

	for i, chunk := range chunks {
		label := fmt.Sprintf("[%d/%d] %d file(s), %s", i+1, total, len(chunk), formatSize(chunkSize(chunk)))

		addArgs := []string{"add", "-A", "--"}
		for _, f := range chunk {
			addArgs = append(addArgs, f.Path)
		}
		if _, err := git(repo, addArgs...); err != nil {
			return err
		}
		message := fmt.Sprintf("%s (%d/%d)", opts.Message, i+1, total)
		if _, err := git(repo, "commit", "-m", message); err != nil {
			return err
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
		if _, err = git(repo, "push", remote, "HEAD:"+branch); err == nil {
			return nil
		}
		if attempt >= retries {
			return fmt.Errorf("push failed after %d attempt(s): %w", attempt+1, err)
		}
		delay := time.Duration(1<<attempt) * time.Second
		logger.Error("push failed (attempt %d/%d), retrying in %s: %v", attempt+1, retries+1, delay, err)
		time.Sleep(delay)
	}
}

// unpushedCommits counts commits on HEAD not on the remote branch.
// Returns 0 when the remote branch doesn't exist yet or on any error.
func unpushedCommits(repo, remote, branch string) int {
	out, err := git(repo, "rev-list", "--count", fmt.Sprintf("%s/%s..HEAD", remote, branch))
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return n
}
