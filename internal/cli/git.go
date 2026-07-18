package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
		if info, err := os.Stat(filepath.Join(repo, path)); err == nil && info.Mode().IsRegular() {
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
	return len(fields) == 0 || fields[0] != head, nil
}

func currentBranch(repo string) (string, error) {
	return git(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
}
