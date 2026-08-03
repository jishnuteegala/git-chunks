# Testing local remotes

The test suite uses a bare repository in a temporary directory as a local remote.
It exercises real `git push` behavior without network access and is also useful
for manual smoke runs.

Create the bare remote, add it as `origin`, and push the initial branch:

```sh
git init --bare -b main remote.git
git remote add origin "$PWD/remote.git"
git push -u origin main
```

Install `remote.git/hooks/pre-receive` to enforce or record pushes. A hook can
reject an oversized push by counting the commits introduced by each ref update:

```sh
#!/bin/sh
while read old new ref; do
  case "$old" in
    0000000000000000000000000000000000000000) count=$(git rev-list --count "$new") ;;
    *) count=$(git rev-list --count "$old..$new") ;;
  esac
  test "$count" -le 1 || exit 1
done
```

Git sends an all-zero old revision when a push creates a branch, so the hook
counts that new branch directly in that case.

To record arrival times, use a hook such as this, replacing the output path with
one outside the bare repository:

```sh
#!/bin/sh
date +%s >>/tmp/git-chunks-pushes
```

For retry/resume testing, make the hook print a diagnostic to stderr and `exit 1`.
Inspect the resulting history with `git -C remote.git rev-list --count main`.

Run this smoke test after building the binary:

```sh
set -eu
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
git init -q -b main "$scratch/work"
git -C "$scratch/work" config user.name smoke
git -C "$scratch/work" config user.email smoke@example.com
git -C "$scratch/work" commit -q --allow-empty -m init
git init -q --bare -b main "$scratch/remote.git"
git -C "$scratch/work" remote add origin "$scratch/remote.git"
git -C "$scratch/work" push -q -u origin main
printf one >"$scratch/work/one.txt"
printf two >"$scratch/work/two.txt"
./git-chunks -C "$scratch/work" -n 1 -p --spacing 1s
git -C "$scratch/work" log --oneline origin/main
```

On Windows, invoke the built binary as `./git-chunks.exe` instead.
