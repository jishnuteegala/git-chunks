# Publishing operations

This guide records the external account configuration and credential maintenance
that cannot be inferred from the repository. It contains no secret values and is
safe to keep under version control.

## Current channels

| Channel | Credential | External state |
|---|---|---|
| GitHub Releases | `GITHUB_TOKEN` | Managed by Actions |
| npm | `NPM_TOKEN` during OIDC migration | Seven packages published at `0.1.0` |
| Homebrew and Scoop | `PACKAGES_GITHUB_TOKEN` | Published from dedicated repositories |
| Winget | `WINGET_GITHUB_TOKEN` | Initial PR awaiting Microsoft review |
| AUR | `AUR_KEY` | `git-chunks-bin` published |
| Chocolatey | `CHOCOLATEY_API_KEY` | `0.1.0` submitted and awaiting moderation |

## npm trusted publishing

npm trusted publishing exchanges GitHub's short-lived OIDC identity for a
single-use publish credential. It requires Node 22.14 or newer, npm 11.5.1 or
newer, a GitHub-hosted runner, and `id-token: write`. The npm job already uses
Node 24, npm 12, and the required permission.

Configure a trusted publisher separately for each package:

- `git-chunks`
- `git-chunks-linux-x64`
- `git-chunks-linux-arm64`
- `git-chunks-darwin-x64`
- `git-chunks-darwin-arm64`
- `git-chunks-windows-x64`
- `git-chunks-windows-arm64`

For each package, open npmjs.com -> Package -> Settings -> Trusted Publisher ->
GitHub Actions and enter:

| Field | Value |
|---|---|
| Organization or user | `jishnuteegala` |
| Repository | `git-chunks` |
| Workflow filename | `release-please.yml` |
| Environment | Leave empty |
| Allowed actions | `npm publish` |

Use `release-please.yml`, not `publish.yml`. npm validates the calling workflow
when a reusable workflow performs the publish. Normal releases call
`publish.yml` from `release-please.yml`, and both workflows grant
`id-token: write`.

Each npm package supports only one trusted publisher. Both normal releases and
manual existing-tag retries enter through `release-please.yml`, so the same OIDC
configuration covers both paths. `publish.yml` is reusable only and cannot be
dispatched directly. The caller also refuses to publish unless the run uses
`refs/heads/main`; when manually dispatching, leave **Use workflow from** set to
`main`.

### Migration sequence

1. Configure all seven trusted publishers with the values above.
2. Keep the current `NPM_TOKEN` for one normal release. npm attempts OIDC before
   falling back to the token, so the token does not prevent testing OIDC.
3. Confirm the new versions show npm provenance linked to this repository and
   `release-please.yml`.
4. On every package, set Publishing access to **Require two-factor
   authentication and disallow tokens**.
5. Revoke the npm access token on npmjs.com, then remove the GitHub secret:

   ```sh
   gh secret delete NPM_TOKEN --repo jishnuteegala/git-chunks
   ```

Do not delete the token before configuring all seven packages. npm does not
validate a trusted publisher when it is saved; configuration errors appear only
on the next real publish.

## Channel credentials

### Homebrew and Scoop

`PACKAGES_GITHUB_TOKEN` is one fine-grained GitHub PAT with access only to
`jishnuteegala/homebrew-tap` and `jishnuteegala/scoop-bucket`. Its repository
permission is **Contents: Read and write**; no account permissions are needed.

### Winget

`WINGET_GITHUB_TOKEN` writes the version branch to the
`jishnuteegala/winget-pkgs` fork and opens or discovers the upstream Microsoft
PR. Use a fine-grained PAT scoped to the fork with **Contents: Read and write**.
If GitHub rejects upstream PR operations with that token, use a classic token
limited to `public_repo`.

### AUR

`AUR_KEY` is an unencrypted, dedicated Ed25519 private key. Its public key must
remain registered in the AUR account. Keep the keypair outside repositories:

```sh
ssh-keygen -t ed25519 -f ~/.ssh/aur_git-chunks_ed25519 -N ""
gh secret set AUR_KEY --repo jishnuteegala/git-chunks < ~/.ssh/aur_git-chunks_ed25519
```

### Chocolatey

`CHOCOLATEY_API_KEY` comes from the Chocolatey account page. The publisher
submits a checksum-pinned package; initial availability depends on Chocolatey
validation, scanning, and moderation.

## Setting secrets

For single-line credentials, let `gh` prompt so values do not enter shell
history:

```sh
gh secret set PACKAGES_GITHUB_TOKEN --repo jishnuteegala/git-chunks
gh secret set WINGET_GITHUB_TOKEN --repo jishnuteegala/git-chunks
gh secret set CHOCOLATEY_API_KEY --repo jishnuteegala/git-chunks
```

List secret names and update times with:

```sh
gh secret list --repo jishnuteegala/git-chunks
```

GitHub never reveals stored secret values. A recent update time proves only
that a value was stored, not that it is valid.

## Monitoring

After every release:

1. Confirm the **Release** workflow completed.
2. Confirm GitHub release assets match `checksums.txt`.
3. Confirm all seven npm versions exist; after OIDC migration, check provenance.
4. Confirm the Homebrew cask and Scoop manifest reference the released version.
5. Confirm the Winget PR is open or merged and its validation checks pass.
6. Confirm the AUR package version, source checksums, and `git` dependency.
7. Confirm Chocolatey is approved and its verifier/scan results pass. A
   successful push means **submitted**, not necessarily publicly installable.

Use **Actions -> Release -> Run workflow** with an existing immutable `v*` tag
to retry. The workflow reuses `release-bundle.tar.gz`, verifies completed
channels, and resumes missing channels. It never creates a new version.

## Rotation and incident response

Review credentials quarterly and after maintainer, repository, or account
changes. Also monitor provider expiry emails and failed publishing jobs.

| Credential | Normal maintenance | Rotation procedure |
|---|---|---|
| npm trusted publisher | Audit all seven package configurations quarterly | No secret rotation; update the publisher immediately if repository/workflow identity changes |
| `NPM_TOKEN` | Remove after OIDC provenance is verified | Token recovery requires temporarily allowing tokens on each affected package, installing a short-lived package-scoped token, recovering the release, then restoring **disallow tokens** and revoking/removing the token |
| `PACKAGES_GITHUB_TOKEN` | Check its provider-side expiry, selected repositories, and Contents permission quarterly | Create replacement, update secret, verify read access with an existing-tag run, prove write access on the next new manifest publication, then revoke old PAT |
| `WINGET_GITHUB_TOKEN` | Check its provider-side expiry, fork scope, and Contents permission quarterly | Create replacement, update secret, verify read access with an existing-tag run, prove write access on the next new Winget branch, then revoke old PAT |
| `AUR_KEY` | Check the registered public keys on AUR quarterly | Generate a new pair, add new public key to AUR, update secret, prove write access on the next package update, then remove old public key and delete old private key |
| `CHOCOLATEY_API_KEY` | Check the Chocolatey account and moderation notifications quarterly | Regenerate on Chocolatey, immediately update the GitHub secret, and prove it on the next new submission |

An existing-tag run is a non-destructive state check. When a version already
exists, publishers intentionally avoid writes, so that run cannot prove that a
replacement PAT, AUR key, Chocolatey key, npm token, or OIDC configuration has
publish authority. Final write validation occurs on the next new version.

If a credential may be exposed, revoke or remove it at the provider first,
rotate it, inspect workflow and provider audit logs, and rerun only after the new
credential is installed. Deleting a GitHub secret alone does not revoke the
credential at its provider.

## Repository controls

- Keep default workflow permissions read-only.
- Prevent Actions from approving pull requests.
- Require maintainer approval before workflows from any external fork run.
- Require `test (ubuntu-latest)`, `test (macos-latest)`,
  `test (windows-latest)`, and `release-checks` on `main`.
- Protect immutable `v*` tags.
- Allow only GitHub-owned actions and explicitly approved third-party actions,
  and enforce full-SHA pinning in repository settings.
- Keep PR CI on `pull_request` with read-only permissions and no secrets. Never
  check out fork code from `pull_request_target` or a privileged `workflow_run`.
- Keep CodeQL default setup enabled for Go, JavaScript/TypeScript, and Actions.
- Keep publishing credentials in GitHub Actions secrets, never in this file.
