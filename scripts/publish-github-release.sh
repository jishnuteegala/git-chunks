#!/usr/bin/env bash
# Uploads the canonical bundle's payloads while the release is a draft, then
# verifies the release assets against the bundle. A published release is only
# verified: immutable releases reject asset changes, and a published release
# must already hold exactly the bundle's payloads.
# Usage: scripts/publish-github-release.sh <tag>
set -euo pipefail

tag=$1
root="$(cd "$(dirname "$0")/.." && pwd)"
fail() { echo "$tag: $*" >&2; exit 1; }

mapfile -t payloads < <(node "$root/scripts/release-payloads.mjs" dist/artifacts.json)
draft=$(gh release view "$tag" --json isDraft --jq .isDraft)
if [ "$draft" = true ]; then
  gh release upload "$tag" --clobber "${payloads[@]}"
else
  echo "$tag is published; verifying its assets without uploading" >&2
fi

assets=$(gh release view "$tag" --json assets --jq '.assets[].name')
if ! diff <(printf '%s\n' "${payloads[@]##*[/\\]}" release-bundle.tar.gz release-bundle.sha256 | sort) <(sort <<< "$assets") >&2; then
  fail "release assets (>) differ from the stored bundle (<)"
fi

checksums="$PWD/dist/checksums.txt"
remote=$(mktemp -d)
trap 'rm -rf "$remote"' EXIT
gh release download "$tag" --dir "$remote" --pattern 'git-chunks_*' --pattern checksums.txt --pattern release-bundle.sha256
cmp -s "$checksums" "$remote/checksums.txt" || fail "release checksums.txt differs from the stored bundle"
cmp -s release-bundle.sha256 "$remote/release-bundle.sha256" || fail "release-bundle.sha256 differs from the stored bundle"
(cd "$remote" && sha256sum --check --quiet "$checksums") >&2 || fail "release assets do not match the stored bundle checksums"
echo "verified $tag release assets against the stored bundle"
