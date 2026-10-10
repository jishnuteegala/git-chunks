# Non-interactive failures

git-chunks runs every Git subprocess with `GIT_TERMINAL_PROMPT=0`, so an
HTTPS remote that would block on a username/password prompt fails fast
with "terminal prompts disabled" instead of hanging the process; SSH
passphrases, host-key confirmations, and configured askpass helpers are
unaffected and must be pre-configured.

## Sub-features

- `https-fail-fast` fails an unauthenticated HTTPS push immediately
  rather than prompting.
- `ssh-unaffected` leaves SSH and askpass behavior to the user's own
  configuration (documented residual, not driven).
- `hooks-inherit` runs local Git hooks under the same non-interactive
  environment.

## How to get to it (user POV)

- Run `git chunks -n 5 -p` against an HTTPS remote with no credential
  helper - the tool exits 1 with the prompt-disabled error instead of
  sitting on a prompt forever.

## Driving it with the shell

Preconditions:

- Doctor passed; `RUN_DIR` with `scratch/` and `evidence/` exists;
  `node` is on `PATH` (the 401 helper needs it); the scratch 401 server
  from `Helpers` is running with its port in `scratch/port.txt` and PID
  in `scratch/server.pid`.
- Git config is isolated so no credential helper can answer: set
  `GIT_CONFIG_NOSYSTEM=1` and point `HOME` (and `XDG_CONFIG_HOME` where
  honored) at an empty `scratch/home` for the driven command.
- A fixture made with `mkrepo creds` provides `scratch/creds/work`; its
  `origin` is repointed at `http://127.0.0.1:$(cat
  scratch/port.txt)/repo.git`.

- **401 fails fast.** With the isolated config, run `git-chunks -C
  scratch/creds/work -n 1 -p` after adding a pending file. The command
  returns exit 1 within seconds - not a hang - and stderr carries
  `terminal prompts disabled`; `git -C scratch/creds/work log` still
  holds the created commits (committed work is preserved for resume).
- **Prompt would fire without the guard.** Sanity-check the fixture:
  `GIT_TERMINAL_PROMPT= git -C scratch/creds/work ls-remote origin`
  against the same 401 server prompts or stalls, proving the endpoint
  really demands credentials.
- **Local remotes unaffected.** The `mkrepo` fixtures push over
  filesystem paths and need no credentials; push drives elsewhere in the
  map keep working under the same environment.

## Gotchas

- A populated credential helper (e.g. manager-core on Windows) masks the
  prompt path - the run needs the isolated Git config in Preconditions or
  the drive proves nothing.
- `GIT_TERMINAL_PROMPT=0` does not block SSH passphrases or askpass
  helpers; verifying a "hang" against `ssh://` or a configured helper is
  out of scope and must not be reported as covered.
- Time-bound the drive mentally: "fails fast" means seconds; anything
  that waits is a defect, not a slow pass.
