# Chunked commits

The core loop: git-chunks takes pending working-tree changes (unstaged
and untracked), groups them into chunks by `--max-files` and/or
`--max-size`, and creates one commit per chunk in order, refusing to run
when the Git index already holds staged changes.

## Sub-features

- `chunk-count` groups at most `-n` files per commit.
- `chunk-size` groups files so each commit stays under `-s` bytes
  (pre-pack working-tree estimate).
- `chunk-oversize` gives a single file larger than `-s` its own commit
  rather than splitting or failing.
- `chunk-message` names each commit `<message> (i/total)` with `chunk`
  as the default prefix.
- `index-guard` refuses a non-empty index with exit 1 before creating
  any commit, and preserves working-tree changes.
- `index-restore` empties the index again if an add or commit fails
  mid-run, keeping the working tree intact.

## How to get to it (user POV)

- Run `git chunks -n <count>` or `git chunks -s <size>` inside a
  repository with pending changes.
- Combine both criteria and a custom prefix: `git chunks -n 100 -s 100M
  -m "import legacy assets"`.

## Driving it with the shell

Preconditions:

- Doctor passed; `RUN_DIR` with `scratch/` and `evidence/` exists; a
  fixture made with `mkrepo demo` provides `scratch/demo/work` and
  `scratch/demo/remote.git`.

- **File-count split.** Create three files in `scratch/demo/work` and
  run `git-chunks -C scratch/demo/work -n 2`. `git -C scratch/demo/work
  log --oneline` shows two commits `chunk (1/2)` and `chunk (2/2)` over
  `init`, and `git status --porcelain` is empty.
- **Message prefix.** Repeat with `-m "import"`: the log shows
  `import (1/N)` style subjects.
- **Index guard.** Stage any file (`git add`), then run `git-chunks -C
  scratch/demo/work -n 1`: exit code 1, stderr reports the staged index,
  `git log` gains no commits, and the staged file is still staged.
- **Oversize single file.** With `-s 1K` and one file of 2 KB plus one
  of 10 B, the run still succeeds: the large file gets its own commit.
- **Untracked and modified.** A mix of a modified tracked file and a new
  untracked file lands across the produced commits; `git status
  --porcelain` ends clean.

## Gotchas

- `--max-size` counts current regular-file bytes on disk: deletions,
  symlinks, submodules, and history contribute nothing, so push size is
  only a heuristic.
- At least one of `-n`/`-s` is required; omitting both exits 2 before
  touching the repository.
- A failed run must restore an empty index - always confirm `git diff
  --cached --name-only` is empty after any failure drive, not just that
  files still exist.
