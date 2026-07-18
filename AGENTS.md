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
- Preserve required check names `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`, and `release-checks` because branch protection depends on them.
- Runtime test jobs leave Go caching disabled because this module has no third-party dependencies and compiles quickly. Release jobs enable the Go module/build cache because they compile the pinned GoReleaser module.

## Project skills

- Use `.agents/skills/develop-git-chunks/SKILL.md` for CLI behavior, Git integration, or platform-sensitive runtime changes.
- Use `.agents/skills/release-git-chunks/SKILL.md` for artifacts, package managers, release automation, or distribution documentation.
