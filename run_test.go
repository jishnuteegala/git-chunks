package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

func TestRunReportsResume(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")

	writeFile(t, repo, "unpushed.txt", 100)
	mustGit(t, repo, "add", "unpushed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "left over from failed run")

	writeFile(t, repo, "new.txt", 100)
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "Resuming: 1 unpushed commit(s)") {
		t.Fatalf("expected resume notice, got: %s", errOut.String())
	}
	if got := commitCount(t, remote, "main"); got != "3" { // init + leftover + new chunk
		t.Fatalf("remote commit count = %s, want 3", got)
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

func TestRunNoCriteriaFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: "."}, &out, &errOut); err == nil {
		t.Fatal("expected error when no criteria given")
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
}

func TestRunNotARepo(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: dir, MaxFiles: 1}, &out, &errOut); err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}
