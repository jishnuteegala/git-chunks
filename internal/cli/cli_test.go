package cli

import (
	"bytes"
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
