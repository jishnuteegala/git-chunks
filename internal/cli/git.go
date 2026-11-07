package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var credentialURL = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s]+@`)

func git(repo string, args ...string) (string, error) {
	out, err := gitRaw(repo, args...)
	return strings.TrimSpace(out), err
}

// gitEnv keeps Git non-interactive: with terminal prompts disabled, a remote
// without a credential helper fails fast instead of blocking on a prompt.
func gitEnv() []string {
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
}

// gitPath resolves git once per process: on Windows each lookup stats every
// PATHEXT extension in every PATH directory ahead of Git's. On failure it
// falls back to the bare name so exec.Command reports its own lookup error.
var gitPath = sync.OnceValue(func() string {
	if path, err := exec.LookPath("git"); err == nil {
		return path
	}
	return "git"
})

func gitCommand(repo string, args ...string) *exec.Cmd {
	cmd := exec.Command(gitPath(), args...)
	// Git sees argv[0] as "git", as with a PATH lookup; the resolved path only starts the process.
	cmd.Args[0] = "git"
	cmd.Dir = repo
	cmd.Env = gitEnv()
	return cmd
}

// gitRaw returns output verbatim; needed for porcelain -z parsing where
// leading spaces in status codes are significant.
func gitRaw(repo string, args ...string) (string, error) {
	out, err := gitCommand(repo, args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(credentialURL.ReplaceAllString(string(out), `${1}[redacted]@`))
		if detail != "" {
			return "", fmt.Errorf("git command failed: %w: %s", err, detail)
		}
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return string(out), nil
}

// pendingFiles lists all uncommitted changes (staged, unstaged, untracked)
// with their on-disk sizes, and reports whether git diff --cached sees staged
// changes. Deleted files report size 0.
func pendingFiles(repo string) (files []File, staged bool, err error) {
	out, err := gitRaw(repo, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, false, err
	}
	entries := strings.Split(out, "\x00")
	seen := map[string]bool{}
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		status, path := entry[:2], entry[3:]
		staged = staged || (status[0] != ' ' && status[0] != '?')
		if strings.ContainsAny(status, "RC") {
			i++ // skip the origin-path entry that follows renames/copies
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, File{Path: path})
	}
	if staged {
		// Status still lists a staged submodule that diff.ignoreSubmodules or
		// submodule.<name>.ignore hides, so git diff --cached has the final say.
		diff, err := gitRaw(repo, "diff", "--cached", "--name-only", "-z")
		if err != nil {
			return nil, false, err
		}
		staged = diff != ""
	}
	statSizes(repo, files)
	return files, staged, nil
}

// statSizes fills in regular-file sizes. Per-file Lstat latency dominates on
// Windows, so a few workers overlap it.
func statSizes(repo string, files []File) {
	const workers = 8
	var wg sync.WaitGroup
	for w := range min(workers, len(files)) {
		wg.Go(func() {
			for i := w; i < len(files); i += workers {
				if info, err := os.Lstat(filepath.Join(repo, files[i].Path)); err == nil && info.Mode().IsRegular() {
					files[i].Size = info.Size()
				}
			}
		})
	}
	wg.Wait()
}

func hasUnpushedCommits(repo, remote, branch string) (bool, error) {
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	out, err := git(repo, "ls-remote", "--heads", remote, "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 || fields[0] == head {
		return len(fields) == 0, nil
	}
	if _, err := git(repo, "fetch", "--quiet", "--no-tags", remote, "refs/heads/"+branch); err != nil {
		return false, err
	}
	remoteHead, err := git(repo, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return false, err
	}
	if gitSuccess(repo, "merge-base", "--is-ancestor", remoteHead, head) {
		return true, nil
	}
	if gitSuccess(repo, "merge-base", "--is-ancestor", head, remoteHead) {
		return false, fmt.Errorf("remote %s/%s is ahead; integrate it before pushing", remote, branch)
	}
	return false, fmt.Errorf("local HEAD and remote %s/%s have diverged; integrate the remote before pushing", remote, branch)
}

func gitSuccess(repo string, args ...string) bool {
	return gitCommand(repo, args...).Run() == nil
}

func currentBranch(repo string) (string, error) {
	return git(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
}
