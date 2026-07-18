#!/usr/bin/env bash
set -euo pipefail

version=$1
branch="git-chunks-$version"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
auth=$(printf 'x-access-token:%s' "$GH_TOKEN" | base64 | tr -d '\n')

git -c http.extraheader="AUTHORIZATION: basic $auth" clone -q "https://github.com/jishnuteegala/winget-pkgs.git" "$work/repo"
if git -C "$work/repo" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
  git -C "$work/repo" checkout -q -B "$branch" "origin/$branch"
else
  git -C "$work/repo" checkout -q -B "$branch"
fi
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
find_pr() {
  curl -fsSL "https://api.github.com/repos/microsoft/winget-pkgs/pulls?state=open&head=jishnuteegala:$branch" |
    node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const h=process.argv[1];const p=JSON.parse(s).find(x=>x.head.sha===h);if(p)process.stdout.write(p.html_url)})' "$head"
}
pr=$(find_pr)
if [ -z "$pr" ]; then
  pr=$(curl -fsSL "https://api.github.com/repos/microsoft/winget-pkgs/pulls?state=closed&head=jishnuteegala:$branch" |
    node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const t=process.argv[1];const p=JSON.parse(s).find(x=>x.merged_at&&x.title===t);if(p)process.stdout.write(p.html_url)})' "$title")
fi
if [ -z "$pr" ]; then
  gh pr create --repo microsoft/winget-pkgs --head "jishnuteegala:$branch" --base master --title "$title" --body "Automated git-chunks release." >/dev/null 2>&1 || true
  for delay in 1 2 4 8 15 30; do
    pr=$(find_pr)
    [ -n "$pr" ] && break
    sleep "$delay"
  done
fi
test -n "$pr"
printf 'verified winget PR: %s\n' "$pr"
