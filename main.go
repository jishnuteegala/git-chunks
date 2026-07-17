// git-chunks commits (and optionally pushes) pending changes in small
// chunks, so every push stays under SCM platform size limits.
package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	var opts Options

	flag.StringVar(&opts.Repo, "C", ".", "path to git repo")
	flag.StringVar(&opts.Repo, "repo", ".", "path to git repo")
	flag.IntVar(&opts.MaxFiles, "n", 0, "max files per commit")
	flag.IntVar(&opts.MaxFiles, "max-files", 0, "max files per commit")
	flag.Var(&opts.MaxSize, "s", "max total size per commit (e.g. 50M, 500K, 1G)")
	flag.Var(&opts.MaxSize, "max-size", "max total size per commit (e.g. 50M, 500K, 1G)")
	flag.StringVar(&opts.Message, "m", "chunk", "commit message prefix")
	flag.StringVar(&opts.Message, "message", "chunk", "commit message prefix")
	flag.BoolVar(&opts.Push, "p", false, "push after each commit")
	flag.BoolVar(&opts.Push, "push", false, "push after each commit")
	flag.StringVar(&opts.Remote, "remote", "origin", "remote to push to")
	flag.StringVar(&opts.Branch, "branch", "", "branch to push (default: current)")
	flag.IntVar(&opts.Retries, "retries", 3, "push retry attempts (exponential backoff)")
	flag.StringVar(&opts.LogFile, "log", "", "append progress to this log file")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "show the plan without committing")
	flag.BoolVar(&opts.JSON, "json", false, "output the --dry-run plan as JSON")
	flag.BoolVar(&opts.Quiet, "q", false, "suppress progress output (errors still shown)")
	flag.BoolVar(&opts.Quiet, "quiet", false, "suppress progress output (errors still shown)")
	showVersion := flag.Bool("version", false, "print version")

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("git-chunks", version)
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "git-chunks: unexpected argument %q\nRun 'git-chunks --help' for usage.\n", flag.Arg(0))
		os.Exit(2)
	}

	if err := Run(opts, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "git-chunks:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `git-chunks %s — commit and push changes in chunks

Splits pending changes into multiple commits based on criteria you set,
optionally pushing after each one, so every push stays under SCM platform
size limits. Safe to rerun: already-committed chunks are skipped.

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
