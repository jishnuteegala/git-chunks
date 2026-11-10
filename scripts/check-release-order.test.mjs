import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import test from "node:test";

const script = join(import.meta.dirname, "check-release-order.mjs");
const check = (...tags) => spawnSync(process.execPath, [script, ...tags], { encoding: "utf8" });

test("a tag older than the latest release fails with a clear message", () => {
  for (const tag of ["v0.2.0", "v0.3.0-rc.1"]) {
    const result = check(tag, "v0.3.0");
    assert.equal(result.status, 1, tag);
    assert.match(result.stderr, new RegExp(`^${tag.replaceAll(".", "\\.")} is older than the latest release v0\\.3\\.0`));
  }
});

test("the latest release tag or a newer tag passes", () => {
  for (const tag of ["v0.3.0", "v0.3.1", "v0.10.0", "v1.0.0-rc.1"]) {
    const result = check(tag, "v0.3.0");
    assert.equal(result.status, 0, `${tag}: ${result.stderr}`);
  }
});

test("a missing latest release tag fails closed", () => {
  assert.equal(check("v0.3.0", "").status, 2);
});
