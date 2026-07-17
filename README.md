# git-chunk

Commit (and optionally push) changes in small chunks to avoid SCM platform push size limits.

## The problem

If you've ever tried to push a large import — vendored dependencies, design assets, a repo migration — you've probably hit one of these:

```text
remote: fatal: pack exceeds maximum allowed size (2.00 GiB)        # GitHub
remote: GitLab: Push size limit exceeded                           # GitLab
error: RPC failed; HTTP 413 curl 22 The requested URL returned error: 413
error: remote unpack failed: error VS403500: size of your push exceeds the limit  # Azure DevOps
```

Most git hosts cap the size of a single push (GitHub: 2 GB/pack, GitLab/Bitbucket/Azure DevOps: configurable, often less). The workaround is always the same tedious loop: stage some files, commit, push, repeat.

`git-chunk` automates that loop. It splits your pending changes into multiple commits based on criteria you set (max files and/or max size per commit), optionally pushing after each one so every push stays under the limit — with retries, resume support, and logging.

Because the binary is named `git-chunk`, git picks it up automatically as a subcommand: `git chunk`.

## Install

```sh
# npm / bun / pnpm
npm install -g git-chunk

# Homebrew (macOS / Linux)
brew install jishnuteegala/tap/git-chunk

# winget (Windows)
winget install jishnuteegala.git-chunk

# Scoop (Windows)
scoop bucket add jishnuteegala https://github.com/jishnuteegala/scoop-bucket
scoop install git-chunk

# Chocolatey (Windows)
choco install git-chunk

# AUR (Arch Linux)
paru -S git-chunk-bin

# Go
go install github.com/jishnuteegala/git-chunk@latest
```

Or grab a prebuilt binary from [Releases](https://github.com/jishnuteegala/git-chunk/releases) — the build matrix covers linux, macOS (`darwin`), and windows on amd64 + arm64.

## Usage

```sh
# 20 files per commit
git chunk -n 20

# max 50 MB per commit, push after each commit
git chunk -s 50M -p

# combine criteria, custom message, preview first
git chunk -n 100 -s 100M -m "import legacy assets" --dry-run

# machine-readable plan, persistent log
git chunk -s 50M --dry-run --json
git chunk -s 50M -p --log push.log
```

## Flags

| Flag | Description |
|------|-------------|
| `-n, --max-files` | Max files per commit |
| `-s, --max-size` | Max total size per commit (`500K`, `50M`, `1G`) |
| `-m, --message` | Commit message prefix (default: `chunk`), suffixed with `(i/total)` |
| `-p, --push` | Push after each commit |
| `--remote` | Remote to push to (default: `origin`) |
| `--branch` | Branch to push (default: current) |
| `--retries` | Push retry attempts with exponential backoff (default: 3) |
| `--dry-run` | Show the chunk plan without committing |
| `--json` | Output the `--dry-run` plan as JSON |
| `--log` | Append timestamped progress to a log file |
| `-q, --quiet` | Suppress progress output (errors still shown) |
| `-C, --repo` | Path to git repo (default: current dir) |
| `--version` | Print version |

At least one of `--max-files` / `--max-size` is required.

## Resumability

`git-chunk` is safe to rerun. Chunks are committed one at a time, so:

- If a push fails (even after retries), committed work is preserved. Rerun the same command — already-committed chunks no longer show as pending, and the unpushed commits ride along with the next push.
- On resume it tells you: `Resuming: N unpushed commit(s) from a previous run will ride along with the first push.`

## Notes

- A single file larger than `--max-size` still gets its own commit — a file can't be split. If it exceeds your platform's hard limit you'll need Git LFS for that file.
- Untracked files are included; deleted files count as 0 bytes.
- Sizes are working-tree sizes; git compresses on push, so actual push sizes are usually smaller than the configured cap.
- Progress goes to stderr; `--dry-run` output goes to stdout (pipe-friendly with `--json`).

## Development

```sh
go test ./...
go build
```

## Releasing

Releases are fully automated with [Conventional Commits](https://www.conventionalcommits.org), [release-please](https://github.com/googleapis/release-please), and [GoReleaser](https://goreleaser.com):

1. Commits to `main` use conventional commit messages (`feat:`, `fix:`, `perf:`, ...)
2. release-please maintains a release PR with the next semver bump and CHANGELOG
3. Merging that PR creates the tag + GitHub release, which triggers GoReleaser to:
   - Build the OS/arch matrix (linux/darwin/windows x amd64/arm64)
   - Attach archives + checksums to the release, with a changelog grouped by type
   - Publish the Homebrew cask to `jishnuteegala/homebrew-tap`
   - Publish the Scoop manifest to `jishnuteegala/scoop-bucket`
   - Open a PR to `microsoft/winget-pkgs` with the winget manifest
   - Publish `git-chunk` + per-platform binary packages to npm
   - Push the Chocolatey `.nupkg` to chocolatey.org (if `CHOCOLATEY_API_KEY` is set)

No manual steps between merging and published packages.

Required repo secrets:

| Secret | Purpose |
|--------|---------|
| `HOMEBREW_TAP_GITHUB_TOKEN` | PAT with write access to the tap + scoop bucket repos |
| `WINGET_GITHUB_TOKEN` | PAT for the `winget-pkgs` fork used to open PRs to microsoft/winget-pkgs |
| `NPM_TOKEN` | npm automation token |
| `CHOCOLATEY_API_KEY` | chocolatey.org API key (optional; step is skipped without it) |
| `AUR_KEY` | SSH private key for the AUR package repo (optional; skipped without it) |

One-time setup: create `homebrew-tap` and `scoop-bucket` repos, fork `microsoft/winget-pkgs`, and note that the first winget and Chocolatey submissions go through manual moderation before automation flows freely.
