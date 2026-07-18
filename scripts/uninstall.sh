#!/bin/sh

set -eu

INSTALL_DIR=${INSTALL_DIR:-"$HOME/.local/bin"}
binary="$INSTALL_DIR/git-chunks"

if [ ! -e "$binary" ] && [ ! -L "$binary" ]; then
  printf 'git-chunks is not installed at %s\n' "$binary"
  exit 0
fi

rm -f "$binary"
printf 'Removed %s\n' "$binary"
