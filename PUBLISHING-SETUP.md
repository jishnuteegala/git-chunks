# Publishing setup guide

What to register for and which tokens to create so the release pipeline can
publish to every channel. Keep this file local — it is gitignored.

## Required before merging the first release PR

### 1. npm (`NPM_TOKEN`, then trusted publishing)

npm's recommended long-term mechanism is **trusted publishing** (OIDC): the
GitHub Actions workflow proves its identity to npm directly — no token to
store, rotate, or leak, and packages get provenance attestations for free.

BUT trusted publishing cannot create a brand-new package — it is configured
per existing package. So this is a two-phase setup:

**Phase 1 — first release only (token):**

- Register at [npmjs.com](https://www.npmjs.com) (free)
- Enable 2FA on the account (required for publishing)
- Generate token: Account -> Access Tokens -> **Granular Access Token**:
  - Expiration: short (e.g. 30 days) is fine — this token is nearly single-use
  - Allowed IP ranges: leave empty (GitHub Actions runners have no fixed IPs)
  - Packages and scopes -> Permissions: **Read and Write**
  - Select packages: **All packages** — unavoidable for the first publish
    because unpublished package names can't be selected
  - Organizations: No access
- Set it: `gh secret set NPM_TOKEN --repo jishnuteegala/git-chunks`
- The names `git-chunks` + 6 platform packages (`git-chunks-linux-x64`,
  `git-chunks-linux-arm64`, `git-chunks-darwin-x64`, `git-chunks-darwin-arm64`,
  `git-chunks-windows-x64`, `git-chunks-windows-arm64`) must be unclaimed —
  check with `npm view git-chunks` (404 = free)

**Phase 2 — after v0.1.0 ships (switch to trusted publishing):**

1. On npmjs.com, for EACH of the 7 packages: Package -> Settings ->
   **Trusted publisher** -> GitHub Actions, with:
   - Organization/user: `jishnuteegala`
   - Repository: `git-chunks`
   - Workflow filename: `publish.yml`
2. Delete the token secret: `gh secret delete NPM_TOKEN --repo jishnuteegala/git-chunks`
3. Done - only the npm job has `id-token: write` permission and
   upgrades npm to a trusted-publishing-capable version; with no token
   present, npm publish authenticates via OIDC automatically.

### 2. Homebrew + Scoop (`PACKAGES_GITHUB_TOKEN`)

- No external registration. The two repos already exist (each package
  manager needs its own repo with its own layout, but one PAT covers both):
  - `jishnuteegala/homebrew-tap` — Homebrew formulas for macOS/Linux [DONE]
  - `jishnuteegala/scoop-bucket` — Scoop manifests for Windows [DONE]
- Remaining: create ONE GitHub **fine-grained PAT**: Settings -> Developer settings ->
  Personal access tokens -> Fine-grained tokens -> Generate new token.
  - Under "Repository access" choose **Only select repositories** and tick
    BOTH repos
  - Under "Permissions" click **+ Add permissions**, pick **Contents** from
    the list, then set its access to **Read and write**
  - "Metadata: Read-only" is added automatically alongside it — expected
  - No account permissions needed
- That single token goes in the `PACKAGES_GITHUB_TOKEN` secret; the
  release config uses it for both the tap and the bucket.

### 3. winget (`WINGET_GITHUB_TOKEN`)

- Fork `microsoft/winget-pkgs` to your account
- Create another fine-grained PAT scoped to your fork (same flow as above:
  "+ Add permissions" -> **Contents** -> Read and write) — or a classic PAT
  with `public_repo`, which is the safe choice if fine-grained gives
  PR-creation trouble against microsoft/winget-pkgs
- First submission gets human-moderated in the winget-pkgs repo; subsequent
  versions usually auto-validate

## Optional (release steps auto-skip without them)

### 4. Chocolatey (`CHOCOLATEY_API_KEY`)

- Register at [chocolatey.org](https://community.chocolatey.org) (free)
- API key is on your account page
- First package goes through moderation (can take days); after approval,
  updates flow automatically

### 5. AUR (`AUR_KEY`)

- Register at [aur.archlinux.org](https://aur.archlinux.org) (free)
- Generate a dedicated SSH keypair in `~/.ssh` (NEVER inside a git repo —
  a private key in a repo folder is one mistake away from being committed):

  ```sh
  ssh-keygen -t ed25519 -f ~/.ssh/aur_git-chunks_ed25519 -N ""
  ```

- Add the **public** key (`~/.ssh/aur_git-chunks_ed25519.pub`) to your AUR
  account: aur.archlinux.org -> My Account -> SSH Public Key
- The **private** key content goes in the `AUR_KEY` secret
- Keep the keypair in `~/.ssh` after setting the secret — you'll need it if
  you ever push to the AUR package repo manually
- Package base: `git-chunks-bin`

## Adding the secrets

Run each command; it prompts `? Paste your secret:` — paste the token value
and press Enter (input is hidden, nothing lands in shell history):

```sh
gh secret set NPM_TOKEN --repo jishnuteegala/git-chunks
gh secret set PACKAGES_GITHUB_TOKEN --repo jishnuteegala/git-chunks
gh secret set WINGET_GITHUB_TOKEN --repo jishnuteegala/git-chunks
gh secret set CHOCOLATEY_API_KEY --repo jishnuteegala/git-chunks   # optional
gh secret set AUR_KEY --repo jishnuteegala/git-chunks < ~/.ssh/aur_git-chunks_ed25519  # optional
```

The `AUR_KEY` line is different: `< file` feeds the private key FILE in
via stdin instead of prompting (multi-line values don't paste well).

Alternative for single-line tokens: `--body "token-value"` sets it inline,
but the value ends up in your shell history — prefer the prompt.

You can also add them in the browser: repo -> Settings -> Secrets and
variables -> Actions -> New repository secret.

Verify with: `gh secret list --repo jishnuteegala/git-chunks`

## Suggested order

1. npm account + token
2. Create `homebrew-tap` and `scoop-bucket` repos + PAT
3. Fork `winget-pkgs` + PAT
4. Add the three required secrets
5. Merge the release PR to ship v0.1.0
6. Add Chocolatey/AUR later — their moderation queues don't block anything

## Retry and verification

Run **Actions -> Publish existing tag -> Run workflow** with the existing
immutable `v*` tag. Per-tag concurrency prevents overlapping retries. The
first run stores one canonical bundle on the draft release. Retries reuse it
instead of rebuilding. GitHub assets must match it exactly, and npm publishes
its preflighted tarballs directly.

- GitHub verifies every expected release asset before downstream channels run.
- npm verifies package metadata and tarball integrity before skipping a version.
- Homebrew and Scoop compare generated manifests and verify the remote commit.
- The GitHub release becomes public after GitHub, npm, Homebrew, and Scoop
  succeed. Winget runs afterward because its validators cannot download draft
  release assets.
- winget verifies that the fork branch has an open or merged upstream PR.

AUR and Chocolatey remain disabled for the first release. Enable them only
after their packages can be generated, checksummed, published, and verified as
independent resumable jobs.

Repository settings are still required outside this codebase: keep default
workflow permissions read-only, require `test (ubuntu-latest)`,
`test (macos-latest)`, `test (windows-latest)`, and `release-checks` on `main`,
enable tag protection for `v*`, and restrict Actions to full-SHA-pinned actions.
