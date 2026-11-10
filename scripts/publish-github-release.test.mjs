import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const script = join(import.meta.dirname, "publish-github-release.sh");
const payloads = ["git-chunks_1.2.3_linux_amd64.tar.gz", "git-chunks_1.2.3_windows_amd64.zip", "git-chunks_1.2.3_linux_amd64.deb"];
const types = ["Archive", "Archive", "Linux Package"];

// A fake gh that serves one release from a directory and logs every call.
const fakeGh = `#!/usr/bin/env bash
set -euo pipefail
unix() { cygpath -u "$1" 2>/dev/null || printf '%s' "$1"; }
remote=$(unix "$GH_SHIM_REMOTE")
printf '%s\\n' "$*" >> "$(unix "$GH_SHIM_LOG")"
case "$1 $2 \${5:-}" in
  "release view isDraft") echo "$GH_SHIM_DRAFT" ;;
  "release view assets") ls -1 "$remote" ;;
  "release upload"*) shift 4; cp "$@" "$remote/" ;;
  "release download"*)
    shift 3
    while [ $# -gt 0 ]; do
      case $1 in
        --dir) dir=$2 ;;
        --pattern) for f in "$remote"/$2; do if [ -e "$f" ]; then cp "$f" "$dir/"; fi; done ;;
      esac
      shift 2
    done ;;
  *) echo "unexpected gh call: $*" >&2; exit 98 ;;
esac
`;

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "publish-github-"));
  const remote = join(root, "remote");
  const shim = join(root, "shim");
  for (const dir of [join(root, "dist"), remote, shim]) mkdirSync(dir);
  writeFileSync(join(shim, "gh"), fakeGh, { mode: 0o755 });
  writeFileSync(join(root, "gh.log"), "");
  const sums = payloads.map((name) => {
    writeFileSync(join(root, "dist", name), `payload ${name}`);
    return `${createHash("sha256").update(`payload ${name}`).digest("hex")}  ${name}\n`;
  });
  writeFileSync(join(root, "dist", "checksums.txt"), sums.join(""));
  writeFileSync(join(root, "dist", "artifacts.json"), JSON.stringify([
    ...payloads.map((name, index) => ({ type: types[index], name, path: `dist/${name}` })),
    { type: "Checksum", name: "checksums.txt", path: "dist/checksums.txt" },
    { type: "Binary", name: "git-chunks", path: "dist/linux_amd64/git-chunks" },
  ]));
  writeFileSync(join(root, "release-bundle.tar.gz"), "bundle");
  writeFileSync(join(root, "release-bundle.sha256"), "bundle checksum\n");
  return { root, remote, shim };
}

// Copies the bundle's assets to the fake release, as a completed upload would.
function publishAll({ root, remote }) {
  for (const name of [...payloads, "checksums.txt"]) writeFileSync(join(remote, name), readFileSync(join(root, "dist", name)));
  for (const name of ["release-bundle.tar.gz", "release-bundle.sha256"]) writeFileSync(join(remote, name), readFileSync(join(root, name)));
}

function run({ root, remote, shim }, draft) {
  const guarded = 'shim=$(cygpath -u "$GH_SHIM_DIR" 2>/dev/null || printf %s "$GH_SHIM_DIR"); PATH="$shim:$PATH"; '
    + '[ "$(command -v gh)" = "$shim/gh" ] || exit 99; exec bash "$0" v1.2.3';
  const result = spawnSync("bash", ["-c", guarded, script], {
    cwd: root,
    encoding: "utf8",
    env: { ...process.env, GH_SHIM_DIR: shim, GH_SHIM_REMOTE: remote, GH_SHIM_LOG: join(root, "gh.log"), GH_SHIM_DRAFT: String(draft) },
  });
  assert.notEqual(result.status, 99, "gh did not resolve to the shim");
  return { ...result, calls: readFileSync(join(root, "gh.log"), "utf8").split("\n").filter(Boolean) };
}

function withFixture(body) {
  const release = fixture();
  try {
    body(release);
  } finally {
    rmSync(release.root, { recursive: true, force: true });
  }
}

test("a draft release gets every payload uploaded with --clobber, then verified", () => withFixture((release) => {
  writeFileSync(join(release.remote, "release-bundle.tar.gz"), "bundle");
  writeFileSync(join(release.remote, "release-bundle.sha256"), "bundle checksum\n");
  writeFileSync(join(release.remote, payloads[0]), "partial upload from an earlier attempt");
  const result = run(release, true);
  assert.equal(result.status, 0, result.stderr);
  const uploads = result.calls.filter((call) => call.startsWith("release upload"));
  assert.equal(uploads.length, 1);
  assert.match(uploads[0], /^release upload v1\.2\.3 --clobber /);
  for (const name of [...payloads, "checksums.txt"]) assert.ok(uploads[0].includes(name), name);
  assert.match(result.stdout, /verified v1\.2\.3 release assets against the stored bundle/);
}));

test("a draft holding an asset the bundle does not have fails after the upload", () => withFixture((release) => {
  writeFileSync(join(release.remote, "release-bundle.tar.gz"), "bundle");
  writeFileSync(join(release.remote, "release-bundle.sha256"), "bundle checksum\n");
  writeFileSync(join(release.remote, "git-chunks_1.2.3_stale.zip"), "from an earlier bundle");
  const result = run(release, true);
  assert.equal(result.status, 1);
  assert.equal(result.calls.filter((call) => call.startsWith("release upload")).length, 1);
  assert.match(result.stderr, /> git-chunks_1\.2\.3_stale\.zip/);
  assert.match(result.stderr, /release assets \(>\) differ from the stored bundle \(<\)/);
}));

test("a published release with the bundle's assets is verified without any upload", () => withFixture((release) => {
  publishAll(release);
  const result = run(release, false);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stderr, /v1\.2\.3 is published; verifying its assets without uploading/);
  assert.ok(result.calls.some((call) => call.startsWith("release download")));
  assert.deepEqual(result.calls.filter((call) => call.startsWith("release upload")), []);
}));

test("a published release that differs from the bundle fails without any upload", () => {
  const cases = [
    ["changed payload", ({ remote }) => writeFileSync(join(remote, payloads[1]), "tampered"), /do not match the stored bundle checksums/],
    ["missing payload", ({ remote }) => rmSync(join(remote, payloads[2])), /release assets \(>\) differ from the stored bundle \(<\)/],
    ["extra asset", ({ remote }) => writeFileSync(join(remote, "git-chunks_1.2.3_extra.zip"), "extra"), /release assets \(>\) differ/],
    ["other checksums.txt", ({ remote }) => writeFileSync(join(remote, "checksums.txt"), "0  other\n"), /checksums\.txt differs from the stored bundle/],
    ["other bundle", ({ remote }) => writeFileSync(join(remote, "release-bundle.sha256"), "other\n"), /release-bundle\.sha256 differs from the stored bundle/],
  ];
  for (const [name, mutate, message] of cases) {
    withFixture((release) => {
      publishAll(release);
      mutate(release);
      const result = run(release, false);
      assert.equal(result.status, 1, name);
      assert.match(result.stderr, message, name);
      assert.deepEqual(result.calls.filter((call) => call.startsWith("release upload")), [], name);
    });
  }
});
