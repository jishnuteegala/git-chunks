#!/usr/bin/env node
const tag = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/;
const [requested, latest] = process.argv.slice(2).map((value) => tag.exec(value));
if (process.argv.length !== 4 || !requested || !latest) {
  console.error("usage: check-release-order.mjs <v* SemVer tag> <latest release v* SemVer tag>");
  process.exit(2);
}

// GitHub's latest release is never a prerelease, so on equal versions only the
// requested tag's prerelease suffix can make it older.
const order = [1, 2, 3].map((part) => Number(requested[part]) - Number(latest[part])).find(Boolean)
  ?? (requested[4] ? -1 : 0);
if (order < 0) {
  console.error(`${requested[0]} is older than the latest release ${latest[0]}; dispatch the latest release tag or a newer draft release tag`);
  process.exit(1);
}
