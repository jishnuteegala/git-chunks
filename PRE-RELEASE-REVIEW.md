# Pre-release review

Target: `v0.1.0`

Status: **code findings addressed; repository gate pending**

## Findings

| Finding | Status | Resolution |
|---|---|---|
| Preserve chunk boundaries and recover failed commits | Addressed | A non-empty index is rejected before mutation. Exact deleted, renamed, modified, and untracked paths are tested. Failed commits restore both normal and unborn indexes without changing the working tree. |
| Make publication retryable and deterministic | Addressed | The first run validates and stores a checksummed canonical bundle on the draft release. Retries verify and reuse it. All channel jobs consume that bundle, and npm publishes its preflighted tarballs directly. |
| Resolve npm binaries from GoReleaser metadata | Addressed | Each platform requires exactly one matching `Binary`; missing, duplicate, and misnamed artifacts fail before publication. |
| Resume partial npm publication | Addressed | All seven packages preflight first. Existing versions must match metadata and integrity. The main package publishes last. Fresh, partial, complete, conflicting, dry-run, and prepared-tarball flows are tested. |
| Require CI on the release PR | External blocker | Configure a `main` ruleset requiring all three `test` jobs and `release-checks`, then update PR #1 to the final commit and confirm those checks pass. |
| Validate documented artifacts | Addressed | CI builds a snapshot, derives expected names from README, validates metadata, rejects duplicates, and recomputes checksums. |
| Handle existing local and remote commits | Addressed | Local-ahead commits push before new chunks. Missing, equal, remote-ahead, and diverged states are distinguished using the remote ref and ancestry. |
| Narrow size and safety claims | Addressed | Documentation describes sizing as a working-tree heuristic and states exclusions, retry boundaries, and trust requirements. Symlinks are excluded with `Lstat`. |
| Validate CLI inputs and dry runs | Addressed | Validation precedes mutation, retries are capped, usage errors return 2, dry runs never push, detached dry runs work, and an explicit destination branch supports detached `HEAD`. |
| Pin release dependencies and minimize privileges | Addressed in code | Actions use full SHAs; GoReleaser and npm use exact versions; jobs receive channel-specific credentials and permissions. Repository SHA enforcement remains an external setting. |
| Make channels observable and recoverable | Addressed | GitHub requires the exact canonical asset set and verifies payload checksums. npm, Homebrew, Scoop, and winget verify remote state independently. Winget accepts only an open PR at the exact fork commit or a merged PR for the exact version. |
| Harden errors and logs | Addressed | Logs use `0600`; Git diagnostics are retained with URL credentials redacted; permanent push errors are not retried. |
| Match concurrency documentation | Addressed | Publication concurrency is scoped to the validated tag. |

## Release Gate

Before merging release PR #1:

1. Push this commit and update the PR head.
2. Add a `main` ruleset requiring `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`, and `release-checks`.
3. Keep default workflow permissions read-only, tag protection for `v*`, and full-SHA action enforcement.
4. Confirm all required checks pass on the exact PR head.
5. Confirm required secrets by name with `gh secret list`; never retrieve their values.

## Local Verification

Run before release:

```sh
go vet ./...
go test -race ./...
gofmt -l cmd internal
node --test scripts/*.test.mjs
go run github.com/goreleaser/goreleaser/v2@v2.17.0 check
git diff --check
```

A GoReleaser snapshot and artifact/npm dry-run also run in CI's
`release-checks` job. Do not publish until the external release gate above is
complete.
