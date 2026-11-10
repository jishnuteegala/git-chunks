#!/usr/bin/env node
const tag = /^v(\d+)\.(\d+)\.(\d+)(?:-([^+]+))?(?:\+.*)?$/;
const [requested, latest] = process.argv.slice(2).map((value) => tag.exec(value));
if (process.argv.length !== 4 || !requested || !latest) {
  console.error("usage: check-release-order.mjs <v* SemVer tag> <latest release v* SemVer tag>");
  process.exit(2);
}

function compareIdentifiers(a, b) {
  const [x, y] = [a, b].map((identifier) => (/^\d+$/.test(identifier) ? Number(identifier) : identifier));
  if (typeof x !== typeof y) return typeof x === "number" ? -1 : 1;
  return x < y ? -1 : x > y ? 1 : 0;
}

// SemVer 2.0.0 precedence; build metadata is ignored.
function precedence(a, b) {
  const core = [1, 2, 3].map((part) => Number(a[part]) - Number(b[part])).find(Boolean);
  if (core) return core;
  if (!a[4] || !b[4]) return (b[4] ? 1 : 0) - (a[4] ? 1 : 0);
  const [x, y] = [a[4].split("."), b[4].split(".")];
  for (let i = 0; i < Math.min(x.length, y.length); i++) {
    const order = compareIdentifiers(x[i], y[i]);
    if (order) return order;
  }
  return x.length - y.length;
}

if (precedence(requested, latest) < 0) {
  console.error(`${requested[0]} is older than the latest release ${latest[0]}; dispatch the latest release tag or a newer draft release tag`);
  process.exit(1);
}
