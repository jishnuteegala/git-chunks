# Dry-run plan

`--dry-run` prints the chunk plan the run would execute - one line per
chunk with its files and pre-pack size estimate, or the same plan as a
JSON array with `--json` - without staging, committing, or pushing
anything.

## Sub-features

- `plan-text` prints human-readable chunk lines and proposed messages on
  stdout, ending with `Dry run: no commits made.`
- `plan-json` emits the plan as a JSON array on stdout; each entry
  carries `index`, `files` (capitalized `Path`/`Size`), `size`,
  `uncompressed_bytes_estimate`, and `proposed_message`.
- `plan-readonly` leaves the index, commits, and working tree untouched
  and warns - on stderr - when staged changes exist instead of refusing.
- `plan-spacing` appends the expected total spacing delay when
  `--spacing` is set.

## How to get to it (user POV)

- Run `git chunks -n 100 -s 100M --dry-run` to preview before committing.
- Run `git chunks --max-size 50M --dry-run --json` in a script and parse
  stdout.

## Driving it with the shell

Preconditions:

- Doctor passed; `RUN_DIR` with `scratch/` and `evidence/` exists; a
  fixture made with `mkrepo plan` provides `scratch/plan/work`.

- **Text plan.** Create files of known sizes and run `git-chunks -C
  scratch/plan/work -n 2 --dry-run`. stdout lists `[i/N]` chunks with
  per-file sizes and ends `Dry run: no commits made.`; `git -C
  scratch/plan/work log --oneline` still shows only `init`.
- **JSON plan.** Run `git-chunks -C scratch/plan/work -n 2 --dry-run
  --json > evidence/plan.json`. The file parses as a JSON array whose
  entries carry the documented keys; `proposed_message` reads
  `chunk (i/N)`.
- **Read-only guarantee.** After any plan drive, `git -C
  scratch/plan/work status --porcelain` still lists the pending files
  and `git diff --cached --name-only` is empty.
- **Staged warning.** Stage a file, then dry-run: exit code stays 0, the
  plan prints, and stderr notes the staged index.
- **Contract errors.** `--json` without `--dry-run` exits 2; running
  with neither `-n` nor `-s` exits 2.

## Gotchas

- Progress/diagnostics share stderr with the staged-index warning;
  capture both streams or the warning is invisible.
- `--json` is only valid with `--dry-run` - proving the pair means
  asserting both the plan file and the usage-error exit code.
- `size` and `uncompressed_bytes_estimate` are the same pre-pack
  estimate; do not treat either as a wire-size guarantee.
