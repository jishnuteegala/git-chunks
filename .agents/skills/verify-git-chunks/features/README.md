# git-chunks verification map

This directory is the maintained source for verifying git-chunks'
user-facing CLI behavior.
Read the baseline before driving, then use the matching feature file as
the recipe.

## Baseline preconditions

- Build once: `go build -o "$RUN_DIR/git-chunks" ./cmd/git-chunks`; the
  binary is `$RUN_DIR/git-chunks` (`.exe` on Windows) or an installed
  `git-chunks` on `PATH` when proving a shipped channel.
- Create a per-run directory `RUN_DIR` with `scratch/` and `evidence/`
  subdirectories, as defined in `../SKILL.md`.
- Build all repositories inside `scratch/` with the `mkrepo` helper;
  never point the CLI at the real checkout or home files.
- Pass the doctor check in `../SKILL.md` before the first drive.

## Driving conventions

- Run every drive from `$RUN_DIR`; recipe paths (`scratch/`,
  `evidence/`) are relative to it.
- Start every recipe from the baseline unless its preconditions say
  otherwise.
- Drive with the documented flags exactly as written; exit codes are part
  of the contract: 0 success, 1 runtime error, 2 usage error.
- stdout carries dry-run plans only (text or `--json`); progress and
  diagnostics ride stderr - capture both or a quiet run can look like it
  produced nothing.
- A drive that pushes proves against the scratch bare remote with
  `git -C remote.git log`/`rev-list`, never a network remote.
- Treat every command literally; quote paths.

## Proof and skip reporting

- Evidence is command transcript (command, stdout, stderr, exit code)
  plus repository state (`git log`, `git status`, remote refs) captured
  after the run.
- A mutation (commits created, refs pushed) needs a read-only second look
  at the repository or remote to count as verified.
- Record the feature ID and the entry point used with every artifact.
- Report an unreachable path with the attempted command and the unmet
  precondition; never report a skipped path as verified elsewhere.
- Cleanup removes `scratch/` and stops helper processes; `evidence/`
  always survives.

## Feature entry contract

Each feature file opens with an H1 and one paragraph on the user-visible
behavior, then exactly four H2 sections in order:

1. `Sub-features` - short IDs, one line per behavior.
2. `How to get to it (user POV)` - every user entry point.
3. `Driving it with <harness>` - `Preconditions:` then labeled
   action/result bullets pairing the user action with the exact command
   and observable result.
4. `Gotchas` - traps that waste or invalidate a run.

## Features

- [Chunked commits](./chunked-commits.md) - split pending changes into
  sequential commits by file count or size.
- [Dry-run plan](./dry-run-plan.md) - preview the chunk plan on stdout as
  text or JSON without committing.
- [Push and resume](./push-and-resume.md) - push each chunk to a remote,
  push existing unpushed commits first, and resume after a failed push.
- [Non-interactive failures](./non-interactive.md) - a remote that would
  prompt for credentials fails fast instead of hanging.
