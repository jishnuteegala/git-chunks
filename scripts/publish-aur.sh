#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: publish-aur.sh <SemVer version>" >&2
  exit 2
fi
: "${AUR_KEY:?AUR_KEY is required}"

source_dir="dist/aur"
pkgbuild="$source_dir/git-chunks-bin.pkgbuild"
srcinfo="$source_dir/git-chunks-bin.srcinfo"
test -f "$pkgbuild"
test -f "$srcinfo"
grep -Fq "pkgver = ${version//-/_}" "$srcinfo"

# Bundles created before AUR publication was enabled omitted the runtime dependency.
if ! grep -Eq "^depends=\(.*'git'.*\)" "$pkgbuild"; then
  sed -i "/^license=/a depends=('git')" "$pkgbuild"
fi
if ! grep -Fq $'\tdepends = git' "$srcinfo"; then
  sed -i $'/^\tlicense = /a\
\tdepends = git' "$srcinfo"
fi

work="$(mktemp -d)"
key="$work/aur_key"
repo="$work/repo"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT
printf '%s\n' "$AUR_KEY" > "$key"
chmod 600 "$key"
export GIT_SSH_COMMAND="ssh -i $key -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -F /dev/null"

git clone --quiet ssh://aur@aur.archlinux.org/git-chunks-bin.git "$repo"
if [[ -f "$repo/.SRCINFO" ]] && grep -Fq "pkgver = ${version//-/_}" "$repo/.SRCINFO"; then
  test "$(git -C "$repo" ls-files | sort)" = $'.SRCINFO\nPKGBUILD'
  cmp "$pkgbuild" "$repo/PKGBUILD"
  cmp "$srcinfo" "$repo/.SRCINFO"
  echo "verified existing AUR git-chunks-bin $version"
  exit 0
fi
remote_version="$(sed -n 's/^[[:space:]]*pkgver = //p' "$repo/.SRCINFO" 2>/dev/null || true)"
if [[ -n "$remote_version" ]] && [[ "$(printf '%s\n%s\n' "${version//-/_}" "$remote_version" | sort -V | tail -n1)" = "$remote_version" ]]; then
  echo "AUR has newer version $remote_version; refusing to replace it with $version" >&2
  exit 1
fi

git -C "$repo" rm -rf --ignore-unmatch . >/dev/null
cp "$pkgbuild" "$repo/PKGBUILD"
cp "$srcinfo" "$repo/.SRCINFO"
git -C "$repo" add PKGBUILD .SRCINFO
git -C "$repo" -c user.name='Jishnu Teegala' -c user.email='134275562+jishnuteegala@users.noreply.github.com' commit --quiet -m "git-chunks v$version"
git -C "$repo" push --quiet origin HEAD:master

remote="$(git -C "$repo" ls-remote origin refs/heads/master | cut -f1)"
test "$remote" = "$(git -C "$repo" rev-parse HEAD)"
echo "published and verified AUR git-chunks-bin $version"
