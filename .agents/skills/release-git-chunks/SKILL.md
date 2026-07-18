---
name: release-git-chunks
description: Maintain git-chunks release and distribution machinery. Use for GoReleaser artifacts, npm binary packages, GitHub Releases, Homebrew, Scoop, Winget, AUR, Chocolatey, or release documentation.
---

# Release git-chunks

1. Classify the change by channel and artifact. Read `.goreleaser.yaml`, `.github/workflows/publish.yml`, and only the channel's publisher script. Treat `dist/` and `npm-stage/` as generated evidence, not source files. Completion: the source configuration, generated output, credential boundary, and remote verification point are identified.

2. Preserve the canonical-bundle chain. The Windows build job creates or reuses one checksummed bundle; every channel consumes that bundle; GitHub assets are verified before manifest channels; the release becomes public after GitHub, npm, Homebrew, and Scoop; Winget runs afterward. Completion: retries cannot silently rebuild, replace, or accept payloads that differ from the stored bundle.

3. Preserve channel-specific guarantees:

   - npm preflights all seven tarballs, validates existing registry metadata and integrity, publishes platform packages before the launcher, and supports partial retries.
   - Homebrew and Scoop compare generated manifests with remote content and verify the resulting remote commit.
   - Winget points only to public immutable release assets and accepts only the exact fork commit represented by an open or merged upstream PR.
   - AUR compares the generated files with the remote Git tree and rejects a conflicting existing version. Chocolatey packages use canonical archive checksums; registry acceptance and public availability are separate states because moderation is asynchronous.

   Completion: the changed channel remains deterministic and a retry distinguishes matching, missing, and conflicting remote state.

4. Keep the platform matrix synchronized across GoReleaser, `scripts/verify-artifacts.mjs`, `scripts/release-payloads.mjs`, npm platform metadata, and README payload names. The supported runtime matrix is Linux/macOS/Windows by amd64/arm64; native Linux packages are `deb`, `rpm`, `apk`, and `pkg.tar.zst`. Completion: every target maps to one correctly named binary/archive and every documented payload is generated and checksummed.

5. Validate release changes with `node --test scripts/*.test.mjs`, `go test ./...`, `go run github.com/goreleaser/goreleaser/v2@v2.17.0 check`, `bash -n` for changed shell publishers, and `git diff --check`. Use snapshot release generation only when artifact output itself changed; never invoke a real publisher from a development branch. Completion: applicable checks pass and no credential or generated release directory is staged.
