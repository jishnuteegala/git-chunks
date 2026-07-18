package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var credentialURL = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s]+@`)

func git(repo string, args ...string) (string, error) {
	out, err := gitRaw(repo, args...)
	return strings.TrimSpace(out), err
}

// gitRaw returns output verbatim; needed for porcelain -z parsing where
// leading spaces in status codes are significant.
func gitRaw(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
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
// with their on-disk sizes. Deleted files report size 0.
func pendingFiles(repo string) ([]File, error) {
	out, err := gitRaw(repo, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	entries := strings.Split(out, "\x00")
	var files []File
	seen := map[string]bool{}
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		status, path := entry[:2], entry[3:]
		if strings.ContainsAny(status, "RC") {
			i++ // skip the origin-path entry that follows renames/copies
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		var size int64
		if info, err := os.Lstat(filepath.Join(repo, path)); err == nil && info.Mode().IsRegular() {
			size = info.Size()
		}
		files = append(files, File{Path: path, Size: size})
	}
	return files, nil
}

func hasStagedChanges(repo string) (bool, error) {
	out, err := gitRaw(repo, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return false, err
	}
	return out != "", nil
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
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	return cmd.Run() == nil
}

func currentBranch(repo string) (string, error) {
	return git(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
}
