package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func initUnbornRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	mustGit(t, repo, "config", "user.name", "test")
	mustGit(t, repo, "config", "user.email", "test@example.com")
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

func TestRunShowsPendingETABeforeFirstPush(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")
	writeFile(t, repo, "a.txt", 100)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "ETA pending") {
		t.Fatalf("stderr missing pending ETA before first push: %s", errOut.String())
	}
}

func TestRunQuietLogsETAWithoutConsoleProgress(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")
	writeFile(t, repo, "a.txt", 100)
	writeFile(t, repo, "b.txt", 100)
	logPath := filepath.Join(t.TempDir(), "chunk.log")

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", Quiet: true, LogFile: logPath}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(errOut.String(), "ETA") {
		t.Fatalf("quiet stderr contains ETA: %s", errOut.String())
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(log), " ETA "); count != 2 {
		t.Fatalf("logged ETA lines = %d, want 2:\n%s", count, log)
	}
}

func TestRunSpacingRequestsInterPushGaps(t *testing.T) {
	for _, test := range []struct {
		name       string
		spacingSet bool
		wantGaps   int
	}{
		{name: "spacing", spacingSet: true, wantGaps: 2},
		{name: "no spacing", wantGaps: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t)
			remote := t.TempDir()
			mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
			mustGit(t, repo, "remote", "add", "origin", remote)
			mustGit(t, repo, "push", "-q", "origin", "main")
			for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
				writeFile(t, repo, name, 100)
			}
			var gaps []time.Duration
			deps := runDeps{
				rand: rand.New(rand.NewSource(1)),
				now:  time.Now,
				sleep: func(_ context.Context, delay time.Duration) error {
					gaps = append(gaps, delay)
					return nil
				},
			}
			var out, errOut bytes.Buffer
			opts := Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", SpacingSet: test.spacingSet, SpacingMin: time.Second, SpacingMax: 2 * time.Second}
			if err := run(context.Background(), opts, &out, &errOut, deps); err != nil {
				t.Fatal(err)
			}
			if len(gaps) != test.wantGaps {
				t.Fatalf("spacing requests = %d, want %d", len(gaps), test.wantGaps)
			}
			for _, gap := range gaps {
				if gap < time.Second || gap > 2*time.Second {
					t.Fatalf("spacing request = %s, want 1s-2s", gap)
				}
			}
			if got := commitCount(t, remote, "main"); got != "4" {
				t.Fatalf("remote commit count = %s, want 4", got)
			}
		})
	}
}

func TestRunSpacingInterruptPreservesCommittedWork(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "push", "-q", "origin", "main")
	writeFile(t, repo, "a.txt", 100)
	writeFile(t, repo, "b.txt", 100)
	ctx, cancel := context.WithCancel(context.Background())
	deps := runDeps{
		rand: rand.New(rand.NewSource(1)),
		now:  time.Now,
		sleep: func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		},
	}
	var out, errOut bytes.Buffer
	opts := Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", SpacingSet: true, SpacingMin: time.Second, SpacingMax: time.Second}
	err := run(ctx, opts, &out, &errOut, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
	if got := commitCount(t, remote, "main"); got != "2" {
		t.Fatalf("remote commit count = %s, want 2", got)
	}
	if got := commitCount(t, repo, "HEAD"); got != "3" {
		t.Fatalf("local commit count = %s, want 3", got)
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
	var gaps []time.Duration
	deps := runDeps{
		rand: rand.New(rand.NewSource(1)),
		now:  time.Now,
		sleep: func(_ context.Context, delay time.Duration) error {
			gaps = append(gaps, delay)
			return nil
		},
	}
	var out, errOut bytes.Buffer
	err := run(context.Background(), Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", SpacingSet: true, SpacingMin: time.Second, SpacingMax: time.Second}, &out, &errOut, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0] != time.Second {
		t.Fatalf("spacing requests = %v, want [1s]", gaps)
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
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho hook-diagnostic >&2\nexit 1\n"), 0o755); err != nil {
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

func TestRunRejectsRemoteAheadAndDiverged(t *testing.T) {
	for _, diverged := range []bool{false, true} {
		name := "ahead"
		if diverged {
			name = "diverged"
		}
		t.Run(name, func(t *testing.T) {
			repo := initRepo(t)
			remote := t.TempDir()
			mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
			mustGit(t, repo, "remote", "add", "origin", remote)
			mustGit(t, repo, "push", "-q", "origin", "main")

			other := t.TempDir()
			mustGit(t, other, "clone", "-q", remote, ".")
			mustGit(t, other, "config", "user.name", "test")
			mustGit(t, other, "config", "user.email", "test@example.com")
			writeFile(t, other, "remote.txt", 10)
			mustGit(t, other, "add", ".")
			mustGit(t, other, "commit", "-q", "-m", "remote")
			mustGit(t, other, "push", "-q")
			if diverged {
				writeFile(t, repo, "local.txt", 10)
				mustGit(t, repo, "add", ".")
				mustGit(t, repo, "commit", "-q", "-m", "local")
			}

			var out, errOut bytes.Buffer
			err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin"}, &out, &errOut)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("Run() error = %v, want %s error", err, name)
			}
		})
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

func TestRunDryRunHumanPlanShowsEstimatesAndProposedMessages(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)
	writeFile(t, repo, "b.txt", 200)

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, Message: "import"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[1/2] 1 file(s), 100 B (pre-pack estimate)",
		"proposed message: import (1/2)",
		"proposed message: import (2/2)",
		"estimates are pre-pack; actual sizes will differ",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("plan missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunDryRunShowsExpectedSpacingRange(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)
	writeFile(t, repo, "b.txt", 100)
	var out, errOut bytes.Buffer
	opts := Options{Repo: repo, MaxFiles: 1, DryRun: true, SpacingSet: true, SpacingMin: time.Second, SpacingMax: 2 * time.Second}
	if err := Run(opts, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "expected total spacing delay: 1s-2s") {
		t.Fatalf("plan missing spacing range: %s", out.String())
	}
}

func TestRunRejectsSpacingWithoutPush(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: ".", MaxFiles: 1, Message: "chunk", SpacingSet: true, SpacingMin: time.Second, SpacingMax: time.Second}, &out, &errOut)
	var usageErr *UsageError
	if !errors.As(err, &usageErr) || usageErr.Message != "--spacing requires --push" {
		t.Fatalf("Run() error = %v, want --spacing requires --push", err)
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
	var plan []planChunk
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON plan %q: %v", out.String(), err)
	}
	if len(plan) != 1 {
		t.Fatalf("plan has %d chunks, want 1", len(plan))
	}
	if plan[0].Size != 100 || plan[0].UncompressedBytesEstimate != plan[0].Size {
		t.Fatalf("plan sizes = %d and %d, want matching 100-byte estimates", plan[0].Size, plan[0].UncompressedBytesEstimate)
	}
	if plan[0].ProposedMessage != "chunk (1/1)" {
		t.Fatalf("proposed message = %q, want %q", plan[0].ProposedMessage, "chunk (1/1)")
	}
}

func TestRunDryRunWarnsAboutStagedChanges(t *testing.T) {
	for _, test := range []struct {
		name        string
		stage       bool
		quiet       bool
		wantWarning bool
	}{
		{name: "unstaged changes", wantWarning: false},
		{name: "staged changes", stage: true, wantWarning: true},
		{name: "staged changes with quiet output", stage: true, quiet: true, wantWarning: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t)
			writeFile(t, repo, "a.txt", 100)
			if test.stage {
				mustGit(t, repo, "add", "a.txt")
			}

			var out, errOut bytes.Buffer
			if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, Quiet: test.quiet}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			gotWarning := strings.Contains(errOut.String(), "note: the index has staged changes; a real run would refuse")
			if gotWarning != test.wantWarning {
				t.Fatalf("staged warning = %t, want %t: %s", gotWarning, test.wantWarning, errOut.String())
			}
		})
	}
}

func TestRunDryRunStagedWarningIsLogged(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", 100)
	mustGit(t, repo, "add", "a.txt")
	logPath := filepath.Join(t.TempDir(), "git-chunks.log")

	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, LogFile: logPath}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	const warning = "note: the index has staged changes; a real run would refuse"
	if !strings.Contains(errOut.String(), warning) {
		t.Fatalf("stderr missing warning: %s", errOut.String())
	}
	if !strings.Contains(string(log), "WARN: "+warning) {
		t.Fatalf("log missing warning: %s", log)
	}
}

func TestRunDryRunPlanIsDeterministic(t *testing.T) {
	for _, test := range []struct {
		name string
		json bool
	}{
		{name: "human"},
		{name: "json", json: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t)
			writeFile(t, repo, "b.txt", 200)
			writeFile(t, repo, "a.txt", 100)

			var first, firstErr, second, secondErr bytes.Buffer
			opts := Options{Repo: repo, MaxFiles: 1, DryRun: true, JSON: test.json}
			if err := Run(opts, &first, &firstErr); err != nil {
				t.Fatal(err)
			}
			if err := Run(opts, &second, &secondErr); err != nil {
				t.Fatal(err)
			}
			if first.String() != second.String() || firstErr.String() != secondErr.String() {
				t.Fatalf("plans differ\nfirst stdout:\n%s\nsecond stdout:\n%s\nfirst stderr:\n%s\nsecond stderr:\n%s", first.String(), second.String(), firstErr.String(), secondErr.String())
			}
		})
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

func TestRunDryRunEmptyHumanPlanOmitsEstimateDisclaimer(t *testing.T) {
	repo := initRepo(t)
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "estimates are pre-pack; actual sizes will differ") {
		t.Fatalf("empty plan includes estimate disclaimer: %s", out.String())
	}
	if out.String() != "Dry run: no commits made.\n" {
		t.Fatalf("empty plan = %q, want only dry-run summary", out.String())
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
		{name: "invalid spacing", opts: Options{Repo: ".", MaxFiles: 1, Message: "chunk", SpacingSet: true, SpacingMin: time.Second, SpacingMax: 0}},
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

func TestETAEstimate(t *testing.T) {
	now := time.Date(2026, time.August, 3, 14, 0, 0, 0, time.UTC)
	remaining := [][]File{{{Size: 100}}, {{Size: 100}}}
	var eta etaEstimate
	if got := eta.message(now, remaining, time.Second, 3*time.Second, true); got != "ETA pending" {
		t.Fatalf("ETA = %q, want pending", got)
	}
	eta.add(10*time.Second, 100)
	if got := eta.message(now, remaining, time.Second, 3*time.Second, true); got != "ETA 24s (completes ~14:00)" {
		t.Fatalf("ETA = %q", got)
	}
	eta.add(20*time.Second, 100)
	if got := eta.message(now, remaining[:1], 0, 0, false); got != "ETA 17s (completes ~14:00)" {
		t.Fatalf("ETA = %q", got)
	}
}

func TestRandomSpacingStaysInRange(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for range 100 {
		got := randomSpacing(r, time.Second, 2*time.Second)
		if got < time.Second || got > 2*time.Second {
			t.Fatalf("random spacing = %s, want 1s-2s", got)
		}
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

func TestRunAllowsDetachedHEADWithExplicitBranch(t *testing.T) {
	repo := initRepo(t)
	remote := t.TempDir()
	mustGit(t, remote, "init", "-q", "--bare", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", remote)
	mustGit(t, repo, "checkout", "--detach", "-q")
	writeFile(t, repo, "new.txt", 10)
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk", Push: true, Remote: "origin", Branch: "detached"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := commitCount(t, remote, "detached"); got != "2" {
		t.Fatalf("remote commit count = %s, want 2", got)
	}
}

func TestRunDryRunAllowsDetachedHEAD(t *testing.T) {
	repo := initRepo(t)
	mustGit(t, repo, "checkout", "--detach", "-q")
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: repo, MaxFiles: 1, DryRun: true, Push: true, Remote: "origin"}, &out, &errOut); err != nil {
		t.Fatal(err)
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
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho hook-diagnostic >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 2, Message: "chunk"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "index was restored") {
		t.Fatalf("Run() error = %v, want restored-index error", err)
	}
	if !strings.Contains(err.Error(), "hook-diagnostic") {
		t.Fatalf("Run() error = %v, want hook diagnostic", err)
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

func TestRunRestoresUnbornIndexAfterCommitFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell hook is not portable to Windows")
	}
	repo := initUnbornRepo(t)
	writeFile(t, repo, "new.txt", 10)
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err := Run(Options{Repo: repo, MaxFiles: 1, Message: "chunk"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "index was restored") {
		t.Fatalf("Run() error = %v, want restored-index error", err)
	}
	if staged := mustGit(t, repo, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("index contains staged files after failure: %q", staged)
	}
	if status := mustGit(t, repo, "status", "--porcelain"); status != "?? new.txt" {
		t.Fatalf("working tree changed after failure: %q", status)
	}
}

func TestPendingFilesExcludesSymlinkTargetSize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	repo := initRepo(t)
	writeFile(t, repo, "target", 100)
	mustGit(t, repo, "add", "target")
	mustGit(t, repo, "commit", "-q", "-m", "target")
	if err := os.Symlink("target", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	files, err := pendingFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "link" || files[0].Size != 0 {
		t.Fatalf("pending files = %#v, want zero-sized link", files)
	}
}

func TestTransientPushError(t *testing.T) {
	for _, test := range []struct {
		message string
		want    bool
	}{
		{"fatal: unable to access: Failed to connect", true},
		{"remote: HTTP 503", true},
		{"rejected: non-fast-forward", false},
		{"authentication failed", false},
	} {
		if got := isTransientPushError(errors.New(test.message)); got != test.want {
			t.Errorf("isTransientPushError(%q) = %v, want %v", test.message, got, test.want)
		}
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

func TestLoggerDiagnosticsFinishTTYETALine(t *testing.T) {
	for name, report := range map[string]func(*Logger){
		"warning": func(l *Logger) { l.Warn("warning") },
		"error":   func(l *Logger) { l.Error("error") },
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			logger, err := NewLogger(&output, "", false)
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()
			logger.ETA("ETA pending", true)
			report(logger)
			if got, want := output.String(), "\rETA pending\n"+name+"\n"; got != want {
				t.Fatalf("logger output = %q, want %q", got, want)
			}
		})
	}
}

func TestLoggerTTYETAClearsPreviousLongerLine(t *testing.T) {
	var output bytes.Buffer
	logger, err := NewLogger(&output, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.ETA("ETA much longer", true)
	logger.ETA("ETA short", true)

	if got, want := output.String(), "\rETA much longer\rETA short      "; got != want {
		t.Fatalf("logger output = %q, want %q", got, want)
	}
}

func TestRunNotARepo(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if err := Run(Options{Repo: dir, MaxFiles: 1}, &out, &errOut); err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}
