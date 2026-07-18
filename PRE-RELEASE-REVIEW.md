# Pre-release adversarial review

Status: **release blocked**

Target: first `v0.1.0` release of `git-chunks`

## Reassessment (2026-07-18)

Status: **release blocked**

This reassessment reviews the current working tree, including uncommitted
changes. The original review remains below as the implementation brief. A
status of **Addressed** means the current implementation and available tests
satisfy the original concern; it does not mean the change has passed CI on the
release PR commit.

### Existing finding status

| # | Finding | Status | Remaining work |
|---|---------|--------|----------------|
| 1 | Preserve chunk boundaries with staged changes | **Partial** | Execution now rejects a non-empty index before committing (`internal/cli/run.go:115-123`) and preserves it in a regression test. Add deleted and renamed-file coverage, verify each commit's exact path list, and test staging/commit failure recovery. A commit failure currently leaves files staged, so a rerun is rejected. |
| 2 | Make release publication retryable by tag | **Partial** | `.github/workflows/publish.yml` adds immutable-tag dispatch, exact checkout, and per-tag concurrency. GoReleaser retries can still fail after a partial asset upload because existing release artifacts are not replaced or verified before re-upload. |
| 3 | Resolve npm artifacts from GoReleaser metadata | **Addressed** | `scripts/publish-npm.mjs:121-147` selects one binary by artifact type, build ID, GOOS, and GOARCH and validates its filename. Tests cover missing and duplicate binaries. |
| 4 | Make the seven-package npm release resumable | **Addressed** | All tarballs are built and inspected before registry access; existing versions require matching metadata and integrity; the main package is last. Fresh, partial, complete, and conflicting states are tested. |
| 5 | Require CI on the release PR | **Partial** | CI has a three-OS matrix and PR #1 has reported green checks, but the remediation in this working tree is not the reviewed PR commit. Confirm the exact release PR head is current and that repository rules require all three checks before merging. |
| 6 | Correct documented Linux artifact names | **Partial** | README names now match the configured template. `scripts/verify-artifacts.mjs` hard-codes a second list instead of deriving documented names from README, and CI does not run a snapshot plus this verifier. |
| 7 | Handle existing unpushed commits | **Addressed** | The remote branch is queried with `git ls-remote`, and existing commits are pushed before new commits. Tests cover success, failure, stale tracking refs, no pending files, and a missing remote branch. |
| 8 | Narrow size and safety guarantees | **Addressed** | README and `llms.txt` describe working-tree size as a heuristic, identify exclusions, narrow resumability, and state the trust assumptions. |
| 9 | Validate CLI inputs and exit-code claims | **Addressed** | Validation precedes repository mutation; usage errors return 2 and runtime failures return 1; invalid sizes and option combinations are tested. See the new retry-count finding below. |
| 10 | Pin privileged release dependencies | **Partial** | Actions, GoReleaser, and npm are pinned and Dependabot covers actions. Repository SHA-pinning enforcement is not enabled, and automation does not update the standalone npm/GoReleaser version pins. |
| 11 | Split publisher credentials by job | **Not addressed** | `.github/workflows/publish.yml:27-89` still gives one job GitHub write, npm OIDC, and all publisher credentials. Split build and channel publishers into least-privilege jobs with immutable artifact handoff. |
| 12 | Make each channel observable and recoverable | **Not addressed** | GitHub, Homebrew, Scoop, winget, and AUR remain one GoReleaser invocation. There is no remote-state verification or independent retry for those channels, npm, or Chocolatey. |
| 13 | Use least-privilege workflow defaults | **Addressed** | CI explicitly uses `contents: read`; release jobs elevate permissions locally. Keep repository defaults read-only and retain branch/tag protection. |
| 14 | Harden script and log handling | **Partial** | npm publication validates SemVer and log creation uses `0600`. Git command errors interpolate all command arguments (`internal/cli/git.go:23`), so a credential-bearing URL supplied as `--remote` can be written to stderr or the log. Redact URL userinfo before reporting commands. |

Current total: **6 addressed, 6 partial, 2 not addressed**. Items 2, 5,
10, 11, and 12 still block release; item 1 needs its remaining correctness
tests before the narrower staged-index contract can be considered complete.

### Fresh adversarial findings

#### A. `--dry-run --push` can mutate the remote

Severity: **High**

References: `internal/cli/run.go:111-135`

The dry-run return only executes when pending files exist. With a clean working
tree and an unpushed commit, `--dry-run --push` continues into the resume path
and pushes that commit. A dry run must never commit or push. The same control
flow makes `--dry-run --json` with no pending files produce no JSON plan.

Required fix:

- Handle every dry run before staged-index checks, unpushed-commit checks, and
  pushes, including the empty plan.
- Add an end-to-end test with a clean working tree, an unpushed commit, and
  `DryRun: true, Push: true`; assert the remote ref is unchanged.
- Assert JSON dry-run output for an empty plan is valid JSON (`[]`).

#### B. A failed commit leaves a state that the command cannot resume

Severity: **Medium**

References: `internal/cli/run.go:149-159`, `internal/cli/run.go:115-123`

Each chunk is staged in the real index before `git commit`. If a hook or commit
failure occurs, the selected paths remain staged. The next invocation rejects
the non-empty index, contradicting the command's general recovery posture and
leaving manual cleanup unexplained.

Required fix:

- Prefer an isolated temporary index, or restore the original empty index if
  staging or committing fails without discarding working-tree changes.
- Add injected failing-hook tests that assert `HEAD`, the index, and the working
  tree remain recoverable.

#### C. Git errors can disclose credentials embedded in remote URLs

Severity: **Medium**

References: `internal/cli/git.go:18-24`, `internal/cli/run.go:195-205`

`gitRaw` includes `strings.Join(args, " ")` in every error. A caller may pass a
remote URL containing userinfo or a token through `--remote`; failed push errors
are then printed and optionally persisted. This violates original item 14.

Required fix:

- Do not render raw Git arguments in user-visible errors, or redact URL
  userinfo and known credential-bearing forms first.
- Add a test proving a sentinel credential is absent from returned errors and
  log output.

#### D. Unbounded retries can overflow backoff and make the CLI effectively hang

Severity: **Medium**

References: `internal/cli/run.go:53-55`, `internal/cli/run.go:195-206`

Validation rejects negative retries but accepts arbitrarily large values. The
`1 << attempt` conversion to `time.Duration` eventually overflows, producing
invalid or wrapped delays, while a huge retry count can keep an automated run
alive indefinitely.

Required fix:

- Define and enforce a practical maximum retry count, or cap the backoff delay
  with overflow-safe arithmetic and a context/deadline.
- Test the accepted boundary and first rejected value.

#### E. Release validation occurs after irreversible publication

Severity: **High**

References: `.github/workflows/publish.yml:67-77`

GoReleaser publishes GitHub assets and package-manager manifests before
`verify-artifacts.mjs` runs. A malformed or incomplete artifact set is detected
only after external state may have changed.

Required fix:

- Build once without publishing, validate artifacts and npm tarballs, then pass
  the checksummed output to isolated publishing jobs.
- Make the validated immutable artifact set the only input to publishers.

#### F. Release-specific tests are not merge gates

Severity: **High**

References: `.github/workflows/ci.yml:17-25`

Required CI runs Go vet, Go tests, and a Go build only. It omits the npm
publisher tests, GoReleaser configuration check, snapshot build, artifact
verification, and a tidy-worktree check. Release code can therefore pass all
required checks and fail only after a tag and release exist.

Required fix:

- Add non-publishing release checks to CI, including
  `node --test scripts/publish-npm.test.mjs`, `goreleaser check`, a snapshot,
  and artifact verification.
- Remove the mutating `go mod tidy` release hook (`.goreleaser.yaml:5-8`) and
  enforce tidy module files in CI with a clean-diff assertion.

#### G. The release is public before publication is complete

Severity: **High**

References: `.github/workflows/release-please.yml:20-35`,
`.goreleaser.yaml:77-81`

Release Please creates the GitHub release before downstream publication. If a
publisher fails, users can discover an empty or partial latest release. Keep it
as a draft until required assets and channels verify, then publish it in an
explicit final step.

#### H. Artifact verification proves names, not contents

Severity: **Medium**

References: `scripts/verify-artifacts.mjs:8-29`

The verifier checks expected basenames and file existence. It does not reject
duplicate names, validate type/build/OS/architecture metadata, recompute
checksums, or prove every distributed payload is covered by `checksums.txt`.

Required fix:

- Validate unique typed manifest entries and platform metadata.
- Recompute and compare checksums for every release payload.
- Verify README-derived filenames rather than maintaining an independent copy.

### Idiomatic Go assessment

The package layout (`cmd/git-chunks` plus `internal/cli`), explicit `Main`
return code, wrapped runtime errors, table-driven validation tests, `0o600`
file mode, and direct use of `exec.Command` are broadly idiomatic. The main Go
concerns are behavioral rather than cosmetic: side effects in dry-run control
flow, index recovery after commit failure, secret-safe error construction, and
unbounded retry/backoff behavior. Avoid adding abstraction solely for style;
fix these at the existing seams and add end-to-end tests.

### Reassessment validation

Passed locally on 2026-07-18:

```sh
go vet ./...
go test -race ./...
gofmt -l cmd internal
node --test scripts/publish-npm.test.mjs
go run github.com/goreleaser/goreleaser/v2@v2.17.0 check
bash -n scripts/publish-npm.sh
```

`gofmt -l` produced no filenames. The passing checks do not cover the new
dry-run mutation or the unresolved remote publication and repository-setting
requirements above.

This document is an implementation brief for the agent addressing the
pre-release review. Do not merge release PR #1 or publish any package until all
items marked **Release blocker** are fixed and their acceptance criteria pass.

The temporary local `.env` file was deleted. No credential was found in Git
history or GitHub secret-scanning alerts, so credential rotation is not part of
this brief.

## Required outcomes

1. The CLI must respect chunk boundaries when the Git index already contains
   staged changes.
2. A failed or partially completed release must be safely retryable for the
   same immutable tag.
3. npm publication must use the current GoReleaser artifacts and tolerate a
   partial previous attempt.
4. The exact release PR commit must pass CI before it can be merged.
5. Documentation must match generated artifact names and must not promise hard
   push-size guarantees the implementation cannot provide.
6. Privileged release dependencies and credentials must have a defensible
   supply-chain security posture.

## Release blockers

### 1. Preserve chunk boundaries with staged changes

Severity: **High**

References: `run.go:84-97`, `git.go:28-57`, `run_test.go`

`pendingFiles` includes staged, unstaged, and untracked changes, but each chunk
is added to the user's existing index and committed with a normal `git commit`.
Files staged for later chunks therefore enter the first commit. The next chunk
can then fail with `nothing to commit`.

Verified reproduction:

1. Commit `a.txt` and `b.txt`.
2. Modify both files.
3. Stage only `b.txt`.
4. Run `git chunks -n 1`.
5. Both changes enter the first commit and the second commit fails.

Implementation direction: use an isolated temporary index through
`GIT_INDEX_FILE` so each commit contains exactly the planned paths while the
original staged/unstaged intent remains safe. If that approach proves
incompatible with the product contract, fail before making changes when the
index is non-empty and document the narrower behavior. Do not silently commit
more than the selected chunk.

Acceptance criteria:

- Add an end-to-end regression test with changes staged for a later chunk.
- Every produced commit respects `--max-files` and the planned path list.
- The command does not fail with an empty later chunk.
- Existing staged, unstaged, deleted, renamed, and untracked changes are
  covered by tests or explicitly rejected before any commit is made.
- Failure during staging or committing leaves recoverable repository state.

### 2. Make release publication retryable by tag

Severity: **Critical**

References: `.github/workflows/release-please.yml:12-69`, especially the
`release_created == 'true'` condition at line 26

Release Please creates the tag and GitHub release before GoReleaser and npm
run. If a later step fails, rerunning the workflow causes Release Please to
report no newly created release, so the publishing job is skipped. This leaves
the release permanently partial without manual surgery.

Implementation direction: separate release creation from publication. Make
publication run from an immutable tag and support an explicit
`workflow_dispatch` retry for an existing tag. Add concurrency scoped to the
tag so two publishers cannot race. Do not move or recreate a published tag.

Acceptance criteria:

- A newly created `v*` tag invokes publication.
- An authorized maintainer can retry publication for an existing tag.
- The retry checks out the exact tag, not the current `main` branch.
- Two runs for the same tag cannot publish concurrently.
- A retry does not require deleting or retagging the GitHub release.
- The workflow documents the recovery command or GitHub UI procedure.

### 3. Resolve npm artifacts from GoReleaser metadata

Severity: **Critical**

References: `.goreleaser.yaml:9-22`, `scripts/publish-npm.sh:13-46`

The script uses obsolete paths such as
`dist/git-chunk_linux_amd64_v1/git-chunks`. The current build ID is
`git-chunks`, and GoReleaser generates paths such as
`dist/git-chunks_linux_amd64_v1/git-chunks`.

GoReleaser does not guarantee its internal distribution-directory naming.
Resolve binary paths from `dist/artifacts.json` by artifact type, build ID,
GOOS, and GOARCH rather than replacing one hard-coded prefix with another.

Acceptance criteria:

- A GoReleaser snapshot followed by an npm packaging dry run finds all six
  platform binaries.
- The six packages contain the correct binary for their declared `os` and
  `cpu` constraints.
- Windows packages contain `git-chunks.exe`; other packages contain
  `git-chunks`.
- Missing, duplicate, or mismatched artifacts fail before any npm package is
  published.

### 4. Make the seven-package npm release resumable

Severity: **High**

References: `scripts/publish-npm.sh:13-60`

The six platform packages and main package are published sequentially. npm
versions are immutable, so a failure after any successful publish makes a
naive retry fail with an existing-version conflict.

Implementation direction: build and inspect all seven tarballs before the
first publish. For each package, query whether the exact version exists. Skip
an existing version only after verifying it is the expected release; otherwise
fail loudly. Keep the main package last so users cannot install a version whose
platform packages have not been attempted.

Acceptance criteria:

- Packaging all seven tarballs is a separate preflight phase.
- No publish starts until every tarball and binary mapping validates.
- Re-running after an interrupted platform-package publish continues safely.
- An existing package version with unexpected metadata or integrity is not
  silently accepted.
- The main `git-chunks` package publishes last.
- Tests or a mock registry exercise fresh, partial, complete, and conflicting
  release states without touching the public registry.

### 5. Require CI on the release PR

Severity: **High**

References: `.github/workflows/ci.yml`, repository branch settings, release PR
#1

PR #1 currently reports `action_required` with no CI jobs, and `main` has no
branch protection or ruleset. The untested release commit can therefore be
merged and immediately published.

Implementation direction: resolve the bot-PR workflow approval issue, then add
a branch ruleset that requires the complete CI matrix before merge. Keep the
workflow permissions needed for Release Please narrowly scoped to its job.

Acceptance criteria:

- CI runs on the current release PR head SHA.
- Linux, macOS, and Windows checks pass for that SHA.
- `main` cannot be merged to while required checks are absent or failing.
- The release workflow cannot substitute post-merge CI for the PR gate.

### 6. Correct documented Linux artifact names

Severity: **High**

References: `README.md:63-78`, `.goreleaser.yaml:32-47`

The README downloads `git-chunk_${VERSION}_...`, while snapshot output uses
`git-chunks_${VERSION}_...`. The documented URLs will return 404.

Acceptance criteria:

- README URLs exactly match the configured nFPM filename template.
- Add a non-publishing artifact test that checks every documented release
  filename against `dist/artifacts.json`.
- Cover `.deb`, `.rpm`, `.apk`, `.pkg.tar.zst`, archives, and checksums.

## Correctness and documentation

### 7. Handle existing unpushed commits before creating more chunks

Severity: **Medium**

References: `run.go:72-105`, `run_test.go:106-129`

Existing unpushed commits currently ride along with the first newly created
chunk. This can exceed the requested threshold and can recreate the same push
that previously failed, plus additional content.

Preferred direction: when `--push` is enabled and the configured upstream has
unpushed commits, push those commits before creating another chunk. If that
push fails, stop without creating new commits. Ensure the selected remote and
branch are compared correctly even when no local remote-tracking ref exists.

Acceptance criteria:

- A resume test proves old unpushed commits are attempted before new commits.
- A failed resume push creates no additional commit.
- A successful resume continues with newly planned chunks.
- New remote branches and missing/stale remote-tracking refs are tested.

### 8. Narrow size and safety guarantees

Severity: **Medium**

References: `README.md:11-33`, `README.md:119-146`, `llms.txt:3-31`,
`git.go:28-57`

Working-tree byte size is a planning heuristic, not a Git pack-size guarantee.
Deleted files, symlinks, submodules, staged content that differs from the work
tree, compression, object history, and protocol overhead can all differ from
the reported value. A single oversized file is also intentionally allowed.

Acceptance criteria:

- Replace statements that every push stays under a hard limit with precise
  heuristic language.
- Define exactly what `--max-size` measures.
- State that server policy findings must be fixed, not bypassed by chunking.
- Narrow “idempotent” and “safe to rerun” to behavior the implementation and
  tests actually guarantee.
- Document that Git hooks, remotes, credential helpers, and repositories must
  be trusted when agents invoke the tool.

### 9. Validate CLI inputs and exit-code claims

Severity: **Medium**

References: `main.go:16-52`, `run.go:35-38`, `run.go:133-145`, `size.go:12-32`

Review all invalid option combinations before making repository changes. In
particular, negative `--max-files`, negative `--retries`, `--json` without
`--dry-run`, detached HEAD with `--push`, empty message/remote/branch values,
and non-finite or overflowing sizes need deterministic behavior. Usage errors
should exit 2 if the documentation claims exit code 2.

Acceptance criteria:

- Add table-driven tests for invalid values and combinations.
- Usage/configuration errors exit 2; repository/runtime failures exit 1.
- Size parsing rejects NaN, infinity, overflow, zero where unusable, and
  unsupported suffixes.
- Negative retries cannot trigger an unintended attempt count.

## Release security and hardening

### 10. Pin privileged release dependencies

Severity: **High**

References: `.github/workflows/release-please.yml:18-48`,
`.github/workflows/ci.yml:15-16`

Release actions use mutable major tags, GoReleaser uses `~> v2`, and the job
installs `npm@latest` while holding publishing credentials.

Acceptance criteria:

- Pin every third-party action to a reviewed full commit SHA with a version
  comment.
- Pin an exact GoReleaser version.
- Pin an exact npm version that supports trusted publishing.
- Enable action SHA-pinning enforcement or restrict allowed actions where the
  repository plan supports it.
- Add dependency automation so upgrades arrive as reviewed pull requests.

### 11. Split publisher credentials by job

Severity: **High**

References: `.github/workflows/release-please.yml:24-69`,
`.goreleaser.yaml:82-172`

One job currently exposes GitHub release access, two package-repository PATs,
the winget PAT, AUR private key, npm credentials/OIDC, and Chocolatey key to a
shared dependency chain.

Acceptance criteria:

- Build once and pass immutable checksummed artifacts to separate publisher
  jobs.
- Each job receives only its own credential and minimum GitHub permissions.
- npm alone receives `id-token: write`.
- Add a protected GitHub release environment if compatible with the desired
  automated flow.
- After npm OIDC is verified for all seven packages, delete `NPM_TOKEN` and set
  npm publishing access to disallow traditional tokens.

### 12. Make each channel observable and recoverable

Severity: **High**

References: `.goreleaser.yaml:82-172`, release workflow

GoReleaser publishes several external channels in one invocation. Some earlier
channels can succeed before a later one fails. GoReleaser also documents that
cross-repository winget PR failures may be logged without failing the pipeline.

Acceptance criteria:

- Verify the expected GitHub assets after upload.
- Verify Homebrew and Scoop commits/manifests after publication.
- Verify the expected winget fork branch and upstream PR; absence must fail
  that channel's job.
- Verify the AUR commit/package base when AUR publishing is enabled.
- Verify all seven npm package versions and the Chocolatey push result.
- Document per-channel retry procedures for an existing tag.

### 13. Use least-privilege workflow defaults

Severity: **Medium**

References: `.github/workflows/ci.yml`, repository Actions settings

The repository default workflow permission is write, and CI declares no
explicit permissions.

Acceptance criteria:

- Set repository default workflow permissions to read-only.
- Add `permissions: contents: read` to CI.
- Grant write permissions only on the jobs that require them.
- Add branch and tag protection appropriate for releases.

### 14. Harden script and log handling

Severity: **Low**

References: `scripts/publish-npm.sh:7-9`, `scripts/publish-npm.sh:52-58`,
`log.go:18-25`

The version argument is interpolated into JavaScript source passed to
`node -e`, and logs are created with mode `0644`.

Acceptance criteria:

- Pass the version as data through an environment variable or process
  argument and validate it as SemVer.
- Create log files with mode `0600` where supported.
- Do not print credential-bearing remote URLs or tokens in errors or logs.

## Environment-file hygiene

The repository now tracks `.env.example` containing names only and ignores
`.env` plus `.env.*`, while explicitly allowing `.env.example`.

Rules for future changes:

- Never put real credentials in `.env.example`.
- Prefer GitHub Actions secrets or a dedicated external secret manager.
- Do not print secret values during tests or diagnostics.
- Keep GitHub secret scanning and push protection enabled.

## Validation commands

Run non-publishing validation before declaring the brief complete:

```sh
go vet ./...
go test -race ./...
go run github.com/goreleaser/goreleaser/v2@<PINNED_VERSION> check
go run github.com/goreleaser/goreleaser/v2@<PINNED_VERSION> release --snapshot --clean --skip=publish
bash -n scripts/publish-npm.sh
node --check npm/git-chunks/bin/git-chunks.js
git diff --check
```

The snapshot command must not access public registries, package repositories,
or production publishing credentials. If Chocolatey tooling is unavailable on
the local platform, run the complete snapshot in CI and retain its artifact
manifest for review.

Also verify repository state:

```sh
gh pr checks 1 --repo jishnuteegala/git-chunks
gh api repos/jishnuteegala/git-chunks/rulesets
gh secret list --repo jishnuteegala/git-chunks
```

Do not print or retrieve secret values.

## Definition of done

- Every release blocker above is implemented and tested.
- All non-publishing validation passes on the three supported operating
  systems.
- Release PR #1 is updated and green at its current head SHA.
- The release can be retried for the same tag without deleting tags, releases,
  or already published package versions.
- Generated package names match README installation commands.
- No workflow job receives credentials or permissions unrelated to its task.
- The agent provides a final channel-by-channel preflight report before the
  maintainer merges PR #1.
