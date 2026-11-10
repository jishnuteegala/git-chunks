import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import test from "node:test";

const script = join(import.meta.dirname, "check-release-order.mjs");
const check = (...tags) => spawnSync(process.execPath, [script, ...tags], { encoding: "utf8" });
// Ascending precedence, from the SemVer 2.0.0 specification's example, plus multi-digit numbers.
const ascending = [
  "v0.2.0", "v0.3.0", "v0.10.0",
  "v1.0.0-alpha", "v1.0.0-alpha.1", "v1.0.0-alpha.beta", "v1.0.0-beta",
  "v1.0.0-beta.2", "v1.0.0-beta.11", "v1.0.0-rc.1", "v1.0.0",
];
const pairs = ascending.slice(1).map((newer, index) => [ascending[index], newer]);

test("a tag older than the latest release fails with a clear message", () => {
  for (const [older, latest] of pairs) {
    const result = check(older, latest);
    assert.equal(result.status, 1, `${older} against ${latest}`);
    assert.ok(result.stderr.startsWith(`${older} is older than the latest release ${latest};`), result.stderr);
  }
});

test("the latest release tag or a newer tag passes", () => {
  for (const [latest, newer] of [...pairs, ...ascending.map((tag) => [tag, tag]), ["v1.0.0", "v1.0.0+build.1"]]) {
    const result = check(newer, latest);
    assert.equal(result.status, 0, `${newer} against ${latest}: ${result.stderr}`);
  }
});

test("malformed tags are usage errors", () => {
  for (const tags of [["v0.3.0", ""], ["0.3.0", "v0.3.0"], ["v0.3.0"]]) {
    assert.equal(check(...tags).status, 2, JSON.stringify(tags));
  }
});
