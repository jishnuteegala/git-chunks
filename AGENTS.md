# Project Agent Guide

## Product invariants

- `git-chunks` is a non-interactive Go CLI that delegates repository operations to the installed `git` executable. It is invoked as either `git-chunks` or Git's external subcommand `git chunks`.
- Chunk size is a planning heuristic based on current regular-file bytes, not Git pack size. Deleted files, symlinks, submodules, history, compression, and protocol overhead do not contribute.
- The command accepts unstaged and untracked changes but rejects a non-empty index before creating commits. A failed add or commit must restore an empty index while preserving working-tree changes.
- With `--push`, existing unpushed commits are pushed before new chunks are created. A failed push preserves completed commits so the same command can resume.
- Stdout is reserved for dry-run plans, including JSON. Progress and diagnostics go to stderr. Exit codes are `0` for success, `1` for runtime errors, and `2` for usage errors.

## Architecture

- `cmd/git-chunks/main.go` is only the version-injected process entry point.
- `internal/cli/cli.go` parses arguments and maps errors to exit codes.
- `internal/cli/run.go` owns validation, orchestration, resumability, and push retries.
- `internal/cli/git.go` is the boundary to Git and porcelain parsing. Preserve NUL-delimited parsing and credential redaction when changing it.
- `internal/cli/chunk.go` contains the deterministic, order-preserving chunk planner.
- The runtime has no third-party Go modules and is built with `CGO_ENABLED=0` for Linux, macOS, and Windows on amd64 and arm64.

## Distribution contract

- `.goreleaser.yaml` defines the six binary archives and eight native Linux packages (`deb`, `rpm`, `apk`, and `pkg.tar.zst` for amd64 and arm64).
- npm is one launcher package plus six OS/CPU-specific binary packages. `scripts/publish-npm.mjs` requires exactly one GoReleaser `Binary` artifact for every platform and publishes the launcher last.
- GitHub Releases hold the canonical checksummed payloads. Homebrew and Scoop publish manifests pointing to those exact assets. Winget submission runs only after the release is public.
- AUR publishes generated `PKGBUILD` and `.SRCINFO` files after the release is public, then verifies the exact remote Git tree. Chocolatey packages reference the canonical Windows archives and verify their SHA256 values; an accepted first submission remains unavailable until community moderation completes.
- Release tags are immutable `v*` SemVer tags. A release retry must reuse and verify `release-bundle.tar.gz`; it must not rebuild a different bundle.

## CI contract

- Third-party actions are pinned to full commit SHAs; the trailing version comments are for Dependabot and human readability.
- Repository settings allow only GitHub-owned and explicitly approved third-party actions, enforce full-SHA pinning, require approval for every external fork workflow, keep tokens read-only by default, and prevent Actions from approving pull requests.
- PR CI uses only `pull_request` with no secrets or write permissions. Do not introduce `pull_request_target`, privileged `workflow_run` processing of PR code/artifacts, or self-hosted runners for untrusted changes.
- Preserve required check names `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`, and `release-checks` because branch protection depends on them.
- Runtime test jobs leave Go caching disabled because this module has no third-party dependencies and compiles quickly. Release jobs enable the Go module/build cache because they compile the pinned GoReleaser module.

## Commit and release-note contract

- Pull requests are squash-merged. Make the PR title the Conventional Commit that should land on `main`; intermediate branch commits do not become release notes.
- Use `fix:` for a user-visible bug fix (SemVer patch), `feat:` for a feature (SemVer minor), and `type!:` plus a `BREAKING CHANGE:` footer for an incompatible change (SemVer major). Use `docs:`, `build:`, and `ci:` for those release-note sections without implying a feature or fix.
- Prefer one independently releasable change per PR. If a PR must contain multiple user-visible fixes or features, put additional complete Conventional Commit messages at the bottom of the squash commit body so Release Please emits each entry; do not flatten them into an inaccurate title.
- Keep implementation-only corrections made within the same PR out of the final release message. Use a `Release-As: x.y.z` footer only for an intentional version override.

## Project skills

- Use `.agents/skills/develop-git-chunks/SKILL.md` for CLI behavior, Git integration, or platform-sensitive runtime changes.
- Use `.agents/skills/release-git-chunks/SKILL.md` for artifacts, package managers, release automation, or distribution documentation.
