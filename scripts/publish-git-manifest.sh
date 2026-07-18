#!/usr/bin/env bash
set -euo pipefail

repo=$1
source=$2
destination=$3
tag=$4
version=${tag#v}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

auth=$(printf 'x-access-token:%s' "$PACKAGES_GITHUB_TOKEN" | base64 | tr -d '\n')
git -c http.extraheader="AUTHORIZATION: basic $auth" clone -q "https://github.com/$repo.git" "$work/repo"
current=$(grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' "$work/repo/$destination" 2>/dev/null | head -n1 || true)
if [ -n "$current" ] && [ "$current" != "$version" ] && [ "$(printf '%s\n%s\n' "$version" "$current" | sort -V | tail -n1)" = "$current" ]; then
  echo "refusing to replace $repo version $current with older version $version" >&2
  exit 1
fi
mkdir -p "$work/repo/$(dirname "$destination")"
cp "$source" "$work/repo/$destination"
git -C "$work/repo" config user.name github-actions
git -C "$work/repo" config user.email github-actions@github.com
git -C "$work/repo" add "$destination"
if ! git -C "$work/repo" diff --cached --quiet; then
  git -C "$work/repo" commit -q -m "git-chunks $tag"
  git -C "$work/repo" -c http.extraheader="AUTHORIZATION: basic $auth" push -q origin HEAD
fi
remote=$(git -C "$work/repo" -c http.extraheader="AUTHORIZATION: basic $auth" ls-remote origin "refs/heads/$(git -C "$work/repo" branch --show-current)" | cut -f1)
test "$remote" = "$(git -C "$work/repo" rev-parse HEAD)"
cmp "$source" "$work/repo/$destination"
