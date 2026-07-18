package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mustGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := git(repo, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	mustGit(t, repo, "config", "user.name", "test")
	mustGit(t, repo, "config", "user.email", "test@example.com")
	mustGit(t, repo, "commit", "--allow-empty", "-m", "init")
	return repo
}

func writeFile(t *testing.T, repo, name string, size int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitCount(t *testing.T, repo, rev string) string {
	t.Helper()
	return mustGit(t, repo, "rev-list", "--count", rev)
}

func TestRunCommitsInChunks(t *testing.T) {
	repo := initRepo(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"} {
		writeFile(t, repo, name, 100)
	}

	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 2, Message: "chunk"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}

	if got := commitCount(t, repo, "HEAD"); got != "4" { // init + 3 chunks
		t.Fatalf("commit count = %s, want 4", got)
	}
	if status := mustGit(t, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("working tree not clean: %q", status)
	}
	last := mustGit(t, repo, "log", "-1", "--format=%s")
	if last != "chunk (3/3)" {
		t.Fatalf("last commit message = %q, want %q", last, "chunk (3/3)")
	}
}

func TestRunChunksBySize(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.bin", 400)
	writeFile(t, repo, "b.bin", 400)
	writeFile(t, repo, "c.bin", 400)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxSize: 1000, Message: "chunk"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, repo, "HEAD"); got != "3" { // init + 2 chunks (400+400, 400)
		t.Fatalf("commit count = %s, want 3", got)
	}
}

func TestRunPushesEachChunk(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, repo, name, 100)
	}

	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "main"); got != "4" { // init + 3 chunks all pushed
		t.Fatalf("remote commit count = %s, want 4", got)
	}
}

func TestRunPushesExistingCommitsBeforeCreatingChunks(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")
	hook := filepath.Join(remote, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nwhile read old new ref; do\n  test $(git rev-list --count $old..$new) -le 1 || exit 1\ndone\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "left over from failed run")

	writeFile(t, repo, "new.txt", 100)
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "main"); got != "3" { // init + leftover + new chunk
		t.Fatalf("remote commit count = %s, want 3", got)
	}
	if parent := mustGit(t, repo, "show", "--format=%s", "--no-patch", "HEAD^"); parent != "left over from failed run" {
		t.Fatalf("new chunk parent = %q, want existing unpushed commit", parent)
	}
}

func TestRunIgnoresStaleRemoteTrackingRef(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "existing unpushed commit")
	mustGit(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	writeFile(t, repo, "new.txt", 100)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "main"); got != "3" {
		t.Fatalf("remote commit count = %s, want 3", got)
	}
}

func TestRunFailedResumePushCreatesNoCommit(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "existing unpushed commit")
	writeFile(t, repo, "new.txt", 100)
	hook := filepath.Join(remote, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	before := mustGit(t, repo, "rev-parse", "HEAD")
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected resume push to fail")
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD changed from %s to %s after failed resume push", before, after)
	}
	if status := mustGit(t, repo, "status", "--porcelain"); status != "?? new.txt" {
		t.Fatalf("working tree changed after failed resume push: %q", status)
	}
}

func TestRunResumePushesWhenNothingRemainsToCommit(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "left over from failed run")

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "main"); got != "2" {
		t.Fatalf("remote commit count = %s, want 2", got)
	}
}

func TestRunPushCreatesMissingRemoteBranch(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	writeFile(t, repo, "new.txt", 100)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", Branch: "new-branch"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "new-branch"); got != "2" {
		t.Fatalf("remote commit count = %s, want 2", got)
	}
}

func TestRunDryRunMakesNoCommits(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, repo, "HEAD"); got != "1" {
		t.Fatalf("commit count = %s, want 1 (dry run must not commit)", got)
	}
	if !strings.Contains(out.String(), "a.txt") {
		t.Fatalf("plan should list a.txt, got: %s", out.String())
	}
}

func TestRunDryRunDoesNotPushExistingCommit(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")
	remoteBefore := mustGit(t, remote, "rev-parse", "main")

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "unpushed")

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, Push: true, Remote: "origin"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if remoteAfter := mustGit(t, remote, "rev-parse", "main"); remoteAfter != remoteBefore {
		t.Fatalf("dry run changed remote from %s to %s", remoteBefore, remoteAfter)
	}
}

func TestRunJSONPlan(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Fatalf("expected JSON array, got: %s", out.String())
	}
}

func TestRunJSONPlanIsEmptyArrayForCleanTree(t *testing.T) {
	repo := initRepo(t)
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var plan []planChunk
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON plan %q: %v", out.String(), err)
	}
	if len(plan) != 0 {
		t.Fatalf("plan has %d chunks, want 0", len(plan))
	}
}

func TestRunNoCriteriaFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: "."}, &out, &errOut); err == nil {
		t.Fatal("expected error when no criteria given")
	}
}

func TestRunRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "negative max files", opts: Options{Repo: ".", MaxFiles: -1, Message: "chunk"}},
		{name: "negative max size", opts: Options{Repo: ".", MaxFiles: 1, MaxSize: -1, Message: "chunk"}},
		{name: "negative retries", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", Retries: -1}},
		{name: "too many retries", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", Retries: maxPushRetries + 1}},
		{name: "json without dry run", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", JSON: true}},
		{name: "empty repo", opts: Options{Repo: " ", MaxFiles: 1, Message: "chunk"}},
		{name: "empty message", opts: Options{Repo: ".", MaxFiles: 1, Message: " "}},
		{name: "empty remote", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", RemoteSet: true}},
		{name: "explicit empty branch", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", BranchSet: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := Run(test.opts, &out, &errOut)
			var usageErr *UsageError
			if err == nil || !errors.As(err, &usageErr) {
				t.Fatalf("Run() error = %v, want UsageError", err)
			}
		})
	}
}

func TestRunAcceptsMaximumRetries(t *testing.T) {
	repo := initRepo(t)
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Retries: maxPushRetries}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsDetachedHEADWithPush(t *testing.T) {
	repo := initRepo(t)
	mustGit(t, repo, "checkout", "--detach", "-q")
	writeFile(t, repo, "new.txt", 100)

	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
	var usageErr *UsageError
	if err == nil || !errors.As(err, &usageErr) {
		t.Fatalf("Run() error = %v, want UsageError", err)
	}
	if got := commitCount(t, repo, "HEAD"); got != "1" {
		t.Fatalf("commit count = %s, want 1", got)
	}
}

func TestRunRejectsAndPreservesStagedChanges(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 10)
	writeFile(t, repo, "b.txt", 10)
	mustGit(t, repo, "add", "a.txt", "b.txt")
	mustGit(t, repo, "commit", "-q", "-m", "files")
	writeFile(t, repo, "a.txt", 20)
	writeFile(t, repo, "b.txt", 30)
	mustGit(t, repo, "add", "b.txt")

	stagedBefore := mustGit(t, repo, "diff", "--cached", "--binary")
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk"}, &out, &errOut); err == nil {
		t.Fatal("expected staged index to be rejected")
	}
	if got := commitCount(t, repo, "HEAD"); got != "2" {
		t.Fatalf("commit count = %s, want 2", got)
	}
	if stagedAfter := mustGit(t, repo, "diff", "--cached", "--binary"); stagedAfter != stagedBefore {
		t.Fatalf("staged changes were modified\nbefore:\n%s\nafter:\n%s", stagedBefore, stagedAfter)
	}
	if status := mustGit(t, repo, "status", "--porcelain"); status != "M a.txt\nM  b.txt" {
		t.Fatalf("working state was not preserved: %q", status)
	}
}

func TestRunRestoresIndexAfterCommitFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell hook is not portable to Windows")
	}
	repo := initRepo(t)
	writeFile(t, repo, "tracked.txt", 10)
	mustGit(t, repo, "add", "tracked.txt")
	mustGit(t, repo, "commit", "-q", "-m", "tracked")
	writeFile(t, repo, "tracked.txt", 20)
	writeFile(t, repo, "untracked.txt", 30)
	statusBefore := mustGit(t, repo, "status", "--porcelain")
	headBefore := mustGit(t, repo, "rev-parse", "HEAD")

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 2, Message: "chunk"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "index was restored") {
		t.Fatalf("Run() error = %v, want restored-index error", err)
	}
	if headAfter := mustGit(t, repo, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Fatalf("HEAD changed from %s to %s", headBefore, headAfter)
	}
	if staged := mustGit(t, repo, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("index contains staged files after failure: %q", staged)
	}
	if statusAfter := mustGit(t, repo, "status", "--porcelain"); statusAfter != statusBefore {
		t.Fatalf("working tree changed after failure: got %q, want %q", statusAfter, statusBefore)
	}
}

func TestRunGitErrorDoesNotDiscloseRemoteCredential(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "unpushed.txt", 10)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "unpushed")
	const credential = "sentinel-secret"
	remote := "https://user:" + credential + "@127.0.0.1:1/repo.git"
	logPath := filepath.Join(t.TempDir(), "git-chunks.log")

	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: remote, LogFile: logPath}, &out, &errOut)
	if err == nil {
		t.Fatal("expected remote operation to fail")
	}
	log, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for name, text := range map[string]string{"error": err.Error(), "stderr": errOut.String(), "log": string(log)} {
		if strings.Contains(text, credential) {
			t.Fatalf("%s disclosed credential: %s", name, text)
		}
	}
}

func TestRunCommitsDeletedRenamedAndUntrackedPathsExactly(t *testing.T) {
	repo := initRepo(t)
	for _, name := range []string{"delete.txt", "rename.txt", "modify.txt"} {
		writeFile(t, repo, name, 10)
	}
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-q", "-m", "files")
	if err := os.Remove(filepath.Join(repo, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, "rename.txt"), filepath.Join(repo, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "modify.txt", 20)
	writeFile(t, repo, "untracked.txt", 30)
	planned, err := pendingFiles(repo)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	wantCommits := len(planned) + 2
	if got := commitCount(t, repo, "HEAD"); got != fmt.Sprint(wantCommits) {
		t.Fatalf("commit count = %s, want %d", got, wantCommits)
	}
	for i, file := range planned {
		rev := fmt.Sprintf("HEAD~%d", len(planned)-1-i)
		paths := strings.Fields(mustGit(t, repo, "show", "--format=", "--name-only", rev))
		if len(paths) != 1 || paths[0] != file.Path {
			t.Fatalf("commit %s paths = %q, want [%s]", rev, paths, file.Path)
		}
	}
}

func TestRunLogFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)
	logPath := filepath.Join(t.TempDir(), "chunk.log")

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", LogFile: logPath}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "committed") {
		t.Fatalf("log file missing progress lines: %s", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("log permissions = %o, want 600", got)
		}
	}
}

func TestRunNotARepo(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: dir, MaxFiles: 1}, &out, &errOut); err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}
