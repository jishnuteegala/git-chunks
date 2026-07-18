#!/usr/bin/env bash
# Packages and publishes all npm packages from GoReleaser's artifact manifest.
# Usage: scripts/publish-npm.sh <version> [--dry-run]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
exec node "$ROOT/scripts/publish-npm.mjs" "$@"
