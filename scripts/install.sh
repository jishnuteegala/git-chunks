#!/bin/sh

set -eu

REPOSITORY="jishnuteegala/git-chunks"
INSTALL_DIR=${INSTALL_DIR:-"$HOME/.local/bin"}
VERSION=${1:-${VERSION:-latest}}

command -v curl >/dev/null 2>&1 || {
  printf '%s\n' "error: curl is required" >&2
  exit 1
}
command -v tar >/dev/null 2>&1 || {
  printf '%s\n' "error: tar is required" >&2
  exit 1
}

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *)
    printf 'error: unsupported operating system: %s\n' "$(uname -s)" >&2
    exit 1
    ;;
esac

case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *)
    printf 'error: unsupported architecture: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

case $VERSION in
latest | stable)
  VERSION=$(curl -fsSL \
    -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/$REPOSITORY/releases/latest" |
    tr -d '\n' |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  [ -n "$VERSION" ] || {
    printf '%s\n' "error: could not determine the latest release" >&2
    exit 1
  }
  ;;
esac

case $VERSION in
  v*) ;;
  [0-9]*) VERSION="v$VERSION" ;;
  *)
    printf 'error: invalid version: %s\n' "$VERSION" >&2
    exit 1
    ;;
esac
printf '%s\n' "$VERSION" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$' || {
  printf 'error: invalid version: %s\n' "$VERSION" >&2
  exit 1
}

number=${VERSION#v}
archive="git-chunks_${number}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPOSITORY/releases/download/$VERSION"
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t git-chunks)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

curl -fsSL "$base_url/$archive" -o "$tmp_dir/$archive"
curl -fsSL "$base_url/checksums.txt" -o "$tmp_dir/checksums.txt"

expected=$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1; exit }' "$tmp_dir/checksums.txt")
[ -n "$expected" ] || {
  printf 'error: %s is missing from checksums.txt\n' "$archive" >&2
  exit 1
}

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')
else
  printf '%s\n' "error: sha256sum or shasum is required" >&2
  exit 1
fi

[ "$actual" = "$expected" ] || {
  printf 'error: checksum verification failed for %s\n' "$archive" >&2
  exit 1
}

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" git-chunks
mkdir -p "$INSTALL_DIR"
cp "$tmp_dir/git-chunks" "$INSTALL_DIR/git-chunks"
chmod 0755 "$INSTALL_DIR/git-chunks"

printf 'Installed git-chunks %s to %s\n' "$VERSION" "$INSTALL_DIR/git-chunks"
case :$PATH: in
  *:"$INSTALL_DIR":*) ;;
  *) printf 'Add %s to PATH to run git-chunks.\n' "$INSTALL_DIR" ;;
esac
