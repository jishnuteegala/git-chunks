# Push and resume

With `-p`, git-chunks pushes existing unpushed commits first, then
creates each chunk commit and pushes it to the remote in order, with
retry backoff and optional `--spacing` pacing; a failed push preserves
committed work so rerunning the same command resumes where it stopped.

## Sub-features

- `push-each` pushes every chunk commit to `--remote`/`--branch` as it
  is created.
- `push-unpushed-first` pushes already-existing unpushed commits before
  creating any new chunk.
- `push-retry` retries transient failures with exponential backoff up to
  `--retries` (default 3, max 6).
- `push-spacing` waits a random delay inside the `--spacing` range
  between chunk pushes.
- `push-resume` keeps committed chunks after a failed push; the next run
  pushes them before chunking what remains.
- `push-diverged` refuses to chunk when the remote is ahead or has
  diverged, telling the user to integrate it first.

## How to get to it (user POV)

- Run `git chunks -s 50M -p` to commit and push each chunk.
- Run `git chunks -s 50M -p --spacing 30s-2m --retries 5` on a host with
  rate limits.
- Rerun the same command after a failure to resume.

## Driving it with the shell

Preconditions:

- Doctor passed; `RUN_DIR` with `scratch/` and `evidence/` exists; a
  fixture made with `mkrepo push` provides `scratch/push/work` with
  `origin` at `scratch/push/remote.git`.

- **Chunked push.** Create two files and run `git-chunks -C
  scratch/push/work -n 1 -p`. `git -C scratch/push/remote.git log
  --oneline main` lists `chunk (2/2)`, `chunk (1/2)`, `init` - remote
  history matches the work repo.
- **Unpushed first.** Commit one local commit manually, add a pending
  file, then run with `-p`: the pre-existing commit lands on the remote
  before the new chunk, and the remote log order proves it.
- **Failed push preserves commits.** Install a `pre-receive` hook in
  `remote.git` that exits 1 (see `docs/testing.md`), run with `-p
  --retries 0`: exit code 1, but `git -C work log` still holds the chunk
  commits. Remove the hook, rerun the same command: the preserved
  commits push and the run completes.
- **Remote ahead.** Commit directly inside `remote.git` (or push from a
  second clone) so the remote moves ahead, then run `-p`: exit code 1,
  stderr says to integrate the remote, no new commits are created.
- **Spacing contract.** With `-p --spacing 1s-1s` and two chunks, the
  run sleeps once between pushes; on `--dry-run` the plan prints the
  expected total spacing delay.

## Gotchas

- Never point `-p` at a network remote; the bare `remote.git` under
  `scratch/` is the only valid target.
- Push failure messages are progress, not data: they land on stderr;
  proof is the remote `git log`, not the progress text.
- `--retries 0` makes failure-injection drives deterministic - without
  it the retry backoff turns a test hook into a slow run.
- `--spacing` requires `--push` (or `--dry-run`); using it alone exits 2.
- A resumed run only chunks what is still pending; count remote commits
  rather than assuming the same plan replays.
