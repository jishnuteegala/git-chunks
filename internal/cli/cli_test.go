package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "valid version", args: []string{"--version"}, want: 0},
		{name: "flag parse error", args: []string{"--max-files", "nope"}, want: 2},
		{name: "unexpected argument", args: []string{"extra"}, want: 2},
		{name: "invalid combination", args: []string{"--max-files", "1", "--json"}, want: 2},
		{name: "runtime failure", args: []string{"--max-files", "1", "--repo", t.TempDir()}, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if got := Main(test.args, &out, &errOut, "test"); got != test.want {
				t.Fatalf("Main() = %d, want %d; stderr: %s", got, test.want, errOut.String())
			}
		})
	}
}

// A git subprocess must fail fast on an HTTP 401 instead of prompting for
// credentials; with GIT_TERMINAL_PROMPT=0 git reports "terminal prompts
// disabled".
func TestGitSubprocessNeverPromptsForCredentials(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	// Isolate ambient credential sources so only the terminal prompt remains.
	emptyConfig := filepath.Join(t.TempDir(), "empty-gitconfig")
	if err := os.WriteFile(emptyConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", emptyConfig)
	t.Setenv("GIT_CONFIG_COUNT", "0")
	for _, env := range []string{"GIT_ASKPASS", "HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "all_proxy", "ALL_PROXY"} {
		t.Setenv(env, "")
	}
	// Git localizes its diagnostics; pin English so the assertion is stable.
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANGUAGE", "")

	_, err := git(t.TempDir(),
		"-c", "credential.helper=",
		"-c", "core.askpass=",
		"ls-remote", server.URL+"/repo.git")
	if err == nil || !strings.Contains(err.Error(), "terminal prompts disabled") {
		t.Fatalf("git ls-remote error = %v, want terminal-prompts-disabled failure", err)
	}
}

func TestRunCLIRejectsEmptyBranch(t *testing.T) {
	var out, errOut bytes.Buffer
	if got := Main([]string{"--max-files", "1", "--branch="}, &out, &errOut, "test"); got != 2 {
		t.Fatalf("Main() = %d, want 2; stderr: %s", got, errOut.String())
	}
}

func TestRunCLIRejectsEmptyMessageAndRemote(t *testing.T) {
	for _, args := range [][]string{
		{"--max-files", "1", "--message="},
		{"--max-files", "1", "--remote="},
	} {
		var out, errOut bytes.Buffer
		if got := Main(args, &out, &errOut, "test"); got != 2 {
			t.Fatalf("Main(%q) = %d, want 2; stderr: %s", args, got, errOut.String())
		}
	}
}

func TestSpacingFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "fixed", args: []string{"--version", "--spacing", "45s"}, want: 0},
		{name: "range", args: []string{"--version", "--spacing", "30s-2m"}, want: 0},
		{name: "invalid duration", args: []string{"--version", "--spacing", "soon"}, want: 2},
		{name: "zero", args: []string{"--version", "--spacing", "0s"}, want: 2},
		{name: "reversed", args: []string{"--version", "--spacing", "2m-30s"}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if got := Main(test.args, &out, &errOut, "test"); got != test.want {
				t.Fatalf("Main() = %d, want %d; stderr: %s", got, test.want, errOut.String())
			}
		})
	}
}
