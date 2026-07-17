#!/usr/bin/env bash
# Publishes per-platform binary packages plus the main git-chunks package.
# Run by the release workflow after goreleaser has built ./dist.
# Usage: scripts/publish-npm.sh <version>
set -euo pipefail

VERSION="${1:?usage: publish-npm.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
STAGE="$ROOT/npm-stage"
rm -rf "$STAGE"

publish_platform() {
  local key="$1" goreleaser_dir="$2"
  local os="${key%-*}" arch="${key#*-}"
  local build_dir="$DIST/$goreleaser_dir"
  local pkg_dir="$STAGE/git-chunks-$key"
  mkdir -p "$pkg_dir/bin"

  local binary="git-chunks"
  [ "$os" = "windows" ] && binary="git-chunks.exe"
  cp "$build_dir/$binary" "$pkg_dir/bin/"

  local node_os="$os"
  [ "$os" = "windows" ] && node_os="win32"

  cat > "$pkg_dir/package.json" <<EOF
{
  "name": "git-chunks-$key",
  "version": "$VERSION",
  "description": "git-chunks binary for $key",
  "repository": "github:jishnuteegala/git-chunks",
  "license": "MIT",
  "os": ["$node_os"],
  "cpu": ["$arch"]
}
EOF
  (cd "$pkg_dir" && npm publish --access public)
}

publish_platform linux-x64     git-chunk_linux_amd64_v1
publish_platform linux-arm64   git-chunk_linux_arm64_v8.0
publish_platform darwin-x64    git-chunk_darwin_amd64_v1
publish_platform darwin-arm64  git-chunk_darwin_arm64_v8.0
publish_platform windows-x64   git-chunk_windows_amd64_v1
publish_platform windows-arm64 git-chunk_windows_arm64_v8.0

main_pkg="$STAGE/git-chunks"
mkdir -p "$main_pkg/bin"
cp "$ROOT/npm/git-chunks/bin/git-chunks.js" "$main_pkg/bin/"
cp "$ROOT/README.md" "$main_pkg/"
node -e "
const pkg = require('$ROOT/npm/git-chunks/package.json');
pkg.version = '$VERSION';
for (const dep of Object.keys(pkg.optionalDependencies)) {
  pkg.optionalDependencies[dep] = '$VERSION';
}
require('fs').writeFileSync('$main_pkg/package.json', JSON.stringify(pkg, null, 2));
"
(cd "$main_pkg" && npm publish --access public)
