---
name: develop-git-chunks
description: Develop git-chunks CLI behavior. Use for argument handling, chunk planning, Git integration, resumability, output, exit codes, or cross-platform runtime changes.
---

# Develop git-chunks

1. Trace the behavior from `internal/cli/cli.go` into `run.go`, then read the narrow implementation file and its tests. Identify whether the change affects the index-preservation, resumability, stdout/stderr, or exit-code contracts in `AGENTS.md`. Completion: the affected contract and test seam are explicit before editing.

2. Add or update the smallest regression test at the owning seam. Use pure unit tests for parsing, sizes, and chunking; use a temporary real Git repository for status, index, commit, branch, remote, or resume behavior. Configure identity inside the temporary repository rather than relying on global Git configuration. Completion: the test fails for the missing behavior or demonstrably covers the changed edge.

3. Implement the behavior at its owner. Keep Git process details in `git.go`, orchestration in `run.go`, argument/exit mapping in `cli.go`, and deterministic grouping in `chunk.go`. Preserve paths returned by NUL-delimited Git porcelain as opaque strings. Completion: the behavior satisfies the contract without introducing an alternate Git abstraction.

4. Validate with `gofmt -w cmd internal`, `go vet ./...`, and `go test -race ./...`. On an OS-specific change, inspect the relevant GoReleaser target and add platform-gated coverage where the behavior truly differs. Completion: all commands pass and every changed product invariant has regression coverage.
