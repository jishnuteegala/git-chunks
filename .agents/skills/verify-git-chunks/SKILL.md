---
name: verify-git-chunks
description: Verify user-facing behavior of git-chunks' CLI (chunked commits, dry-run plans, chunked push with resume, non-interactive failures) by building the real binary, driving scratch Git repositories, capturing transcripts and remote history as evidence, and cleaning up scratch state. Use for /verify-git-chunks, "verify the CLI", or "prove this feature works" on this repository.
---

# Verify git-chunks

Prove git-chunks' user-facing features work by driving the CLI's real paths
against disposable Git repositories and capturing evidence a reviewer can
trust.

Read `features/README.md` for baseline preconditions and driving conventions,
then follow the matching feature file as the recipe for each drive.

## Launch

- Build the binary once per session: `go build -o "$RUN_DIR/git-chunks"
  ./cmd/git-chunks` (`git-chunks.exe` on Windows). An installed `git-chunks`
  on `PATH` is an acceptable substitute when the goal is proving a shipped
  channel rather than the checkout.
- git-chunks is a short-lived CLI: every drive is one process invocation
  against a scratch repository. There is no server, daemon, port, or
  instance to keep alive.
- Per-run isolation contract:
  - Give every verification run its own run directory:
    `RUN_DIR="${TMPDIR:-/tmp}/git-chunks-verify-$(date +%Y%m%d%H%M%S)"` with
    `scratch/` for repositories and `evidence/` for artifacts.
  - Build all Git fixtures inside `scratch/` with the `mkrepo` helper under
    `Helpers`; never run the CLI against the real checkout or any repo you
    did not create.
  - For the credential-prompt drive, start the scratch 401 server under
    `Helpers` and stop it in cleanup.
- Teardown is per-invocation; nothing persists between commands except the
  scratch repositories you created.

## Doctor

Run a read-only health check before any drive:

```sh
BIN="$RUN_DIR/git-chunks"        # or `git-chunks` when testing an installed channel
"$BIN" --version                 # expect `git-chunks dev` (local build) or `git-chunks <semver>`
"$BIN" --help >/dev/null 2>&1    # expect exit 0; confirms the flag tree is usable
git --version                    # git-chunks shells out to git; it must exist on PATH
mkdir -p "$RUN_DIR/scratch" "$RUN_DIR/evidence"
echo doctor-ok > "$RUN_DIR/evidence/doctor.txt"
```

Doctor passes when `--version` prints `git-chunks dev` or the released
semver, `--help` exits 0, and `git` resolves. A failed doctor means the
build or checkout is broken; do not drive.

## Drive

- Drive only through the CLI surface: `git-chunks` (or `git chunks`). No
  internal Go APIs, test hooks, or environment poking beyond the documented
  flags and the Git isolation variables named in the feature files.
- Prefer stable handles: long flags and documented output text, not
  terminal coordinates or column positions.
- Every drive runs a real user path inside `scratch/` repositories created
  by `mkrepo`; point the binary at them with `-C <path>` or `cd`.
- Capture each invocation's stdout, stderr, and exit code as it happens;
  exit codes are part of the contract: 0 success, 1 runtime error, 2 usage
  error.
- stdout is reserved for dry-run plans (including `--json`); progress and
  diagnostics go to stderr. A clean plan drive prints on stdout only.

## Evidence

- Write every artifact under `$RUN_DIR/evidence/`: command transcripts
  (command + stdout + stderr + exit code), captured `--dry-run` and
  `--json` output, `git log`/`rev-list` output from the scratch work repo
  and its bare remote, and `git status` snapshots proving the index
  contract.
- A drive is proven only when the evidence shows both the user action and
  the resulting state: the command that ran and the commits, remote refs,
  or plan output it produced.
- Mutation proof needs a read-only second look: after a chunked commit or
  push, inspect `git -C work log` or `git -C remote.git log` rather than
  trusting the progress lines.
- If a path is unreachable (no network remote permitted, platform lacks a
  prerequisite), record the attempted command and the unmet precondition as
  evidence; do not report it verified through a different path.
- The evidence directory must survive `Cleanup`.

## Cleanup

- Stop the scratch 401 server if one was started (record its PID at
  launch; on Windows use `Stop-Process -Id <pid>`; elsewhere `kill <pid>`).
- Remove `$RUN_DIR/scratch/` only. Never delete `$RUN_DIR/evidence/`.
- Do not kill processes by name; a correctly launched run owns at most the
  one optional helper PID.
- After cleanup, confirm `ls "$RUN_DIR/evidence"` still lists artifacts.

## Helpers

- Scratch repository with bare remote (`mkrepo <name>`): creates
  `scratch/<name>/work` with committed `init`, `scratch/<name>/remote.git`
  as `origin`, and pushes `main` - the fixture every repository drive
  starts from:

  ```sh
  mkrepo() {
    local name="$1"
    local base="$RUN_DIR/scratch/$name"
    git init -q -b main "$base/work"
    git -C "$base/work" config user.name verify
    git -C "$base/work" config user.email verify@example.com
    git -C "$base/work" commit -q --allow-empty -m init
    git init -q --bare -b main "$base/remote.git"
    git -C "$base/work" remote add origin "$base/remote.git"
    git -C "$base/work" push -q -u origin main
  }
  ```

- Scratch 401 server (credential-prompt drive only): returns HTTP 401 to
  every request, so a `http://127.0.0.1:<port>` remote demands credentials
  no terminal can supply:

  ```sh
  node -e 'require("http").createServer((q,s)=>{s.writeHead(401);s.end()}).listen(0,"127.0.0.1",function(){console.log(this.address().port)})' \
    > "$RUN_DIR/scratch/port.txt" & echo $! > "$RUN_DIR/scratch/server.pid"
  ```

  Wait for `port.txt` to be non-empty before driving against it.
- Evidence transcript helper (per drive):

  ```sh
  run_capture() { # label, then command...
    local label="$1"; shift
    { echo "+ $*"; "$@" >"$RUN_DIR/evidence/$label.stdout" 2>"$RUN_DIR/evidence/$label.stderr"; echo "exit=$?"; } \
      > "$RUN_DIR/evidence/$label.transcript" 2>&1
  }
  ```

Keep this skill honest: run `/maintain-verification-skill` when CLI flags,
exit codes, or feature behavior change.
