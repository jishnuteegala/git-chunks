// git-chunks commits (and optionally pushes) pending changes in small
// chunks, reducing the amount of new work sent in each push.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// Main parses command-line arguments and executes git-chunks.
func Main(args []string, stdout, stderr io.Writer, version string) int {
	var opts Options
	flags := flag.NewFlagSet("git-chunks", flag.ContinueOnError)
	flags.SetOutput(stderr)

	flags.StringVar(&opts.Repo, "C", ".", "path to git repo")
	flags.StringVar(&opts.Repo, "repo", ".", "path to git repo")
	flags.IntVar(&opts.MaxFiles, "n", 0, "max files per commit")
	flags.IntVar(&opts.MaxFiles, "max-files", 0, "max files per commit")
	flags.Var(&opts.MaxSize, "s", "max total size per commit (e.g. 50M, 500K, 1G)")
	flags.Var(&opts.MaxSize, "max-size", "max total size per commit (e.g. 50M, 500K, 1G)")
	opts.Message = "chunk"
	setMessage := func(value string) error {
		opts.Message = value
		opts.MessageSet = true
		return nil
	}
	flags.Func("m", "commit message prefix", setMessage)
	flags.Func("message", "commit message prefix", setMessage)
	flags.BoolVar(&opts.Push, "p", false, "push after each commit")
	flags.BoolVar(&opts.Push, "push", false, "push after each commit")
	opts.Remote = "origin"
	flags.Func("remote", "remote to push to", func(value string) error {
		opts.Remote = value
		opts.RemoteSet = true
		return nil
	})
	flags.Func("branch", "branch to push (default: current)", func(value string) error {
		opts.Branch = value
		opts.BranchSet = true
		return nil
	})
	flags.IntVar(&opts.Retries, "retries", 3, "push retry attempts (exponential backoff)")
	flags.StringVar(&opts.LogFile, "log", "", "append progress to this log file")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "show the plan without committing")
	flags.BoolVar(&opts.JSON, "json", false, "output the --dry-run plan as JSON")
	flags.BoolVar(&opts.Quiet, "q", false, "suppress progress output (errors still shown)")
	flags.BoolVar(&opts.Quiet, "quiet", false, "suppress progress output (errors still shown)")
	showVersion := flags.Bool("version", false, "print version")

	flags.Usage = func() { usage(stderr, version) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "git-chunks: unexpected argument %q\nRun 'git-chunks --help' for usage.\n", flags.Arg(0))
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, "git-chunks", version)
		return 0
	}

	if err := Run(opts, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "git-chunks:", err)
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			return 2
		}
		return 1
	}
	return 0
}

func usage(out io.Writer, version string) {
	fmt.Fprintf(out, `git-chunks %s — commit and push changes in chunks

Splits pending changes into multiple commits based on criteria you set,
  optionally pushing after each one. Chunk sizes are planning heuristics, not
  guarantees about Git's compressed wire size. Completed chunks are safe to
  resume after a push failure.

Usage:
  git chunks [flags]

Chunking (at least one required):
  -n, --max-files <n>    max files per commit
  -s, --max-size <size>  max total size per commit (e.g. 50M, 500K, 1G)

Committing and pushing:
  -m, --message <msg>    commit message prefix (default: "chunk")
  -p, --push             push after each commit
      --remote <name>    remote to push to (default: origin)
      --branch <name>    branch to push (default: current branch)
      --retries <n>      push retry attempts with backoff (default: 3)

Output:
      --dry-run          show the chunk plan without committing
      --json             print the --dry-run plan as JSON (for scripts)
      --log <file>       append timestamped progress to a log file
  -q, --quiet            suppress progress output (errors still shown)

Other:
  -C, --repo <path>      path to git repo (default: current dir)
      --version          print version
  -h, --help             show this help

Examples:
  git chunks -n 20                     20 files per commit
  git chunks -s 50M -p                 max 50 MB per commit, push each
  git chunks -n 100 -s 100M --dry-run  preview the plan
  git chunks -s 50M -p --log push.log  keep a persistent progress log

Docs: https://github.com/jishnuteegala/git-chunks
`, version)
}
