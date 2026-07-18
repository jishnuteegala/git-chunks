#!/usr/bin/env bash
set -euo pipefail

version=$1
branch="git-chunks-$version"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
auth=$(printf 'x-access-token:%s' "$GH_TOKEN" | base64 | tr -d '\n')

git -c http.extraheader="AUTHORIZATION: basic $auth" clone -q "https://github.com/jishnuteegala/winget-pkgs.git" "$work/repo"
git -C "$work/repo" checkout -q -B "$branch"
destination="manifests/j/jishnuteegala/git-chunks/$version"
rm -rf "$work/repo/$destination"
mkdir -p "$work/repo/$destination"
cp dist/winget/manifests/j/jishnuteegala/git-chunks/"$version"/* "$work/repo/$destination/"
git -C "$work/repo" config user.name github-actions
git -C "$work/repo" config user.email github-actions@github.com
git -C "$work/repo" add "$destination"
if ! git -C "$work/repo" diff --cached --quiet; then git -C "$work/repo" commit -q -m "git-chunks v$version"; fi
expected=$(git -C "$work/repo" -c http.extraheader="AUTHORIZATION: basic $auth" ls-remote origin "refs/heads/$branch" | cut -f1)
git -C "$work/repo" -c http.extraheader="AUTHORIZATION: basic $auth" push -q --force-with-lease="refs/heads/$branch:$expected" origin "$branch"
head=$(git -C "$work/repo" rev-parse HEAD)
title="New version: jishnuteegala.git-chunks version $version"
pr=$(gh pr list --repo microsoft/winget-pkgs --head "jishnuteegala:$branch" --state open --json url,headRefOid --jq ".[] | select(.headRefOid == \"$head\") | .url")
if [ -z "$pr" ]; then
  pr=$(gh pr list --repo microsoft/winget-pkgs --state merged --search "\"$title\" in:title" --json url,title --jq ".[] | select(.title == \"$title\") | .url" | head -n1)
fi
if [ -z "$pr" ]; then
  pr=$(gh pr create --repo microsoft/winget-pkgs --head "jishnuteegala:$branch" --base master --title "$title" --body "Automated git-chunks release.")
fi
test -n "$pr"
printf 'verified winget PR: %s\n' "$pr"
