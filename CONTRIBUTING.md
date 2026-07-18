# Contributing

Thanks for improving `git-chunks`. Bug reports, focused fixes, tests, and
documentation updates are welcome.

## Before you start

- Search existing issues and pull requests first.
- Open an issue before a large feature or behavior change so the approach can
  be agreed before significant work.
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Development

Requirements: Git and the Go version declared in `go.mod`. Node.js 24 is needed
only for release-script tests.

```sh
git clone https://github.com/jishnuteegala/git-chunks.git
cd git-chunks
go test ./...
```

Keep changes small and add regression tests for behavior changes. Before opening
a pull request, run:

```sh
gofmt -w cmd internal
go vet ./...
go test -race ./...
node --test scripts/*.test.mjs  # when changing release or npm scripts
git diff --check
```

Do not run publishing workflows, create release tags, or use real credentials
from a contribution branch.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org): `fix:`
  produces a patch release, `feat:` produces a minor release, and a `!` plus a
  `BREAKING CHANGE:` footer produces a major release. Use `docs:`, `build:`, or
  `ci:` when those more accurately describe the release note.
- Keep unrelated changes in separate pull requests.
- Explain the problem, the chosen solution, and how it was tested.
- Update documentation when behavior or user-facing output changes.
- Resolve review conversations and keep required checks green.

Pull requests are squash-merged, so the PR title must also be a valid
Conventional Commit message and should describe the change users will see on
`main`, not an implementation step. Prefer one independently releasable change
per pull request. If one pull request necessarily contains multiple fixes or
features, add each extra complete Conventional Commit message at the bottom of
the pull request body as raw text, outside a Markdown code fence, with no
validation results or other content after it. GitHub uses the pull request body
as the squash commit body in this repository. Before merging, verify the squash
dialog still ends with those messages, as required by
[Release Please](https://github.com/googleapis/release-please#what-if-my-pr-contains-multiple-fixes-or-features).
