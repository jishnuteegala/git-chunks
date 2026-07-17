# git-chunks

[![CI](https://github.com/jishnuteegala/git-chunks/actions/workflows/ci.yml/badge.svg)](https://github.com/jishnuteegala/git-chunks/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jishnuteegala/git-chunks?include_prereleases)](https://github.com/jishnuteegala/git-chunks/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/jishnuteegala/git-chunks.svg)](https://pkg.go.dev/github.com/jishnuteegala/git-chunks)
[![Go Report Card](https://goreportcard.com/badge/github.com/jishnuteegala/git-chunks)](https://goreportcard.com/report/github.com/jishnuteegala/git-chunks)
[![npm](https://img.shields.io/npm/v/git-chunks)](https://www.npmjs.com/package/git-chunks)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-fe5196?logo=conventionalcommits&logoColor=white)](https://www.conventionalcommits.org)

Split large pushes into chunked commits to beat SCM push size limits and per-push scan timeouts.

## The problem

If you've ever tried to push a large import — vendored dependencies, design assets, a repo migration — you've probably hit one of these:

```text
remote: fatal: pack exceeds maximum allowed size (2.00 GiB)        # GitHub
remote: GitLab: Push size limit exceeded                           # GitLab
error: RPC failed; HTTP 413 curl 22 The requested URL returned error: 413
error: remote unpack failed: error VS403500: size of your push exceeds the limit  # Azure DevOps
remote: pre-receive hook declined                                  # timed-out server-side scan
error: RPC failed; HTTP 500 curl 22 The requested URL returned error: 500  # server timed out processing the push
```

A single oversized push fails for two distinct reasons:

- **Hard size limits** — most hosts cap the size of one push (GitHub: 2 GB/pack; GitLab/Bitbucket/Azure DevOps: configurable, often much less).
- **Per-push scan and hook timeouts** — secret scanning, push protection, malware/DLP scanning, and custom pre-receive hooks all run against each push, and are killed after a time budget. A push with thousands of files or gigabytes of content can blow that budget, and the whole push is rejected even though it's under the size limit. Enterprise-managed GitLab/GitHub instances are especially prone to this.

The workaround is always the same tedious loop: stage some files, commit, push, repeat.

`git-chunks` automates that loop. It splits your pending changes into multiple commits based on criteria you set (max files and/or max size per commit), optionally pushing after each one — so every push stays under the size limit *and* gives server-side scans a small, fast workload. With retries, resume support, and logging.

Because the binary is named `git-chunks`, git picks it up automatically as a subcommand: `git chunks`.

## Install

```sh
# npm / bun / pnpm
npm install -g git-chunks

# Homebrew (macOS / Linux)
brew install jishnuteegala/tap/git-chunks

# winget (Windows)
winget install jishnuteegala.git-chunks

# Scoop (Windows)
scoop bucket add jishnuteegala https://github.com/jishnuteegala/scoop-bucket
scoop install git-chunks

# Chocolatey (Windows)
choco install git-chunks

# AUR (Arch Linux)
paru -S git-chunks-bin

# Go
go install github.com/jishnuteegala/git-chunks@latest
```

### Linux packages

`git-chunks` is not in the official Debian/Fedora/etc. archives, so plain `apt install git-chunks` won't work. Instead, every release attaches native packages you download and install manually. For example (amd64):

```sh
VERSION=$(curl -s https://api.github.com/repos/jishnuteegala/git-chunks/releases/latest | grep -Po '"tag_name": "v\K[^"]*')

# Debian / Ubuntu
curl -LO "https://github.com/jishnuteegala/git-chunks/releases/download/v${VERSION}/git-chunk_${VERSION}_linux_amd64.deb"
sudo dpkg -i "git-chunk_${VERSION}_linux_amd64.deb"

# Fedora / RHEL
sudo dnf install "https://github.com/jishnuteegala/git-chunks/releases/download/v${VERSION}/git-chunk_${VERSION}_linux_amd64.rpm"
```

`.apk` (Alpine) and `.pkg.tar.zst` (Arch) packages are also attached to each [release](https://github.com/jishnuteegala/git-chunks/releases); Arch users should prefer the AUR package above, which handles updates. Note these manual installs don't auto-update — Homebrew, npm, or the AUR are better if you want upgrades handled for you.

Prebuilt binary archives are also on the Releases page — the build matrix covers linux, macOS (`darwin`), and windows on amd64 + arm64.

## Usage

```sh
# 20 files per commit
git chunks -n 20

# max 50 MB per commit, push after each commit
git chunks -s 50M -p

# combine criteria, custom message, preview first
git chunks -n 100 -s 100M -m "import legacy assets" --dry-run

# machine-readable plan, persistent log
git chunks -s 50M --dry-run --json
git chunks -s 50M -p --log push.log
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

## Usage for AI agents and scripts

`git-chunks` is non-interactive, idempotent, and safe to rerun. The recipe:

```sh
# 1. Preview the plan as JSON (stdout; progress goes to stderr)
git chunks --max-size 50M --dry-run --json

# 2. Execute: commit in chunks and push each one, with retries and a log
git chunks --max-size 50M --push --retries 3 --log git-chunks.log
```

- Exit codes: `0` success, `1` runtime error, `2` usage error.
- If a push fails after retries, committed work is preserved — rerun the exact same command to resume.
- A machine-readable summary of this tool lives in [`llms.txt`](llms.txt).

## Resumability

`git-chunks` is safe to rerun. Chunks are committed one at a time, so:

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
   - Publish `git-chunks` + per-platform binary packages to npm
   - Push the Chocolatey `.nupkg` to chocolatey.org (if `CHOCOLATEY_API_KEY` is set)

No manual steps between merging and published packages.

Required repo secrets:

| Secret | Purpose |
|--------|---------|
| `PACKAGES_GITHUB_TOKEN` | PAT with write access to the tap + scoop bucket repos |
| `WINGET_GITHUB_TOKEN` | PAT for the `winget-pkgs` fork used to open PRs to microsoft/winget-pkgs |
| `NPM_TOKEN` | npm automation token |
| `CHOCOLATEY_API_KEY` | chocolatey.org API key (optional; step is skipped without it) |
| `AUR_KEY` | SSH private key for the AUR package repo (optional; skipped without it) |

One-time setup: create `homebrew-tap` and `scoop-bucket` repos, fork `microsoft/winget-pkgs`, and note that the first winget and Chocolatey submissions go through manual moderation before automation flows freely.

## License

MIT — see [LICENSE](LICENSE).
