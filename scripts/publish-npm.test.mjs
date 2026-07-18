import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repository = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const publisher = join(repository, "scripts", "publish-npm.mjs");
const mockNpm = join(repository, "scripts", "mock-npm.mjs");
const platformPackages = [
  ["linux", "amd64", "linux-x64", "git-chunks"],
  ["linux", "arm64", "linux-arm64", "git-chunks"],
  ["darwin", "amd64", "darwin-x64", "git-chunks"],
  ["darwin", "arm64", "darwin-arm64", "git-chunks"],
  ["windows", "amd64", "windows-x64", "git-chunks.exe"],
  ["windows", "arm64", "windows-arm64", "git-chunks.exe"],
];

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "publish-npm-"));
  mkdirSync(join(root, "dist"), { recursive: true });
  mkdirSync(join(root, "npm", "git-chunks", "bin"), { recursive: true });
  writeFileSync(join(root, "README.md"), "fixture\n");
  writeFileSync(join(root, "npm", "git-chunks", "bin", "git-chunks.js"), "#!/usr/bin/env node\n");
  writeFileSync(join(root, "npm", "git-chunks", "package.json"), JSON.stringify({
    name: "git-chunks",
    version: "0.0.0",
    bin: { "git-chunks": "bin/git-chunks.js" },
    optionalDependencies: Object.fromEntries(platformPackages.map(([, , key]) => [`git-chunks-${key}`, "0.0.0"])),
  }));

  const artifacts = platformPackages.map(([goos, goarch, key, binary]) => {
    const path = join("dist", key, binary);
    mkdirSync(join(root, dirname(path)), { recursive: true });
    writeFileSync(join(root, path), `${goos}/${goarch}`);
    return { type: "Binary", name: binary, path, goos, goarch, extra: { ID: "git-chunks" } };
  });
  writeFileSync(join(root, "dist", "artifacts.json"), JSON.stringify(artifacts));
  return root;
}

function run(root, state, ...args) {
  return spawnSync(process.execPath, [publisher, ...args], {
    encoding: "utf8",
    env: {
      ...process.env,
      PUBLISH_NPM_ROOT: root,
      PUBLISH_NPM_CLI: process.execPath,
      PUBLISH_NPM_CLI_SCRIPT: mockNpm,
      MOCK_NPM_STATE: state,
    },
  });
}

function published(root) {
  try {
    return readFileSync(join(root, "npm-calls.log"), "utf8").trim().split("\n").filter(Boolean);
  } catch {
    return [];
  }
}

test("fresh release preflights all packages and publishes main last", () => {
  const root = fixture();
  try {
    const result = run(root, "fresh", "1.2.3");
    assert.equal(result.status, 0, result.stderr);
    const calls = published(root);
    assert.equal(calls.length, 7);
    assert.equal(calls.at(-1), "git-chunks-1.2.3.tgz");
    for (const [, , key, binary] of platformPackages) {
      assert.equal(readFileSync(join(root, "npm-stage", `git-chunks-${key}`, "bin", binary), "utf8"),
        `${key.startsWith("windows") ? "windows" : key.split("-")[0]}/${key.endsWith("x64") ? "amd64" : "arm64"}`);
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("partial release verifies existing packages and resumes", () => {
  const root = fixture();
  try {
    const result = run(root, "partial", "1.2.3");
    assert.equal(result.status, 0, result.stderr);
    assert.equal(published(root).length, 5);
    assert.equal(published(root).at(-1), "git-chunks-1.2.3.tgz");
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("complete release verifies all packages without publishing", () => {
  const root = fixture();
  try {
    const result = run(root, "complete", "1.2.3");
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(published(root), []);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("conflicting existing package fails before any publish", () => {
  const root = fixture();
  try {
    const result = run(root, "conflict", "1.2.3");
    assert.equal(result.status, 1);
    assert.match(result.stderr, /unexpected dist\.integrity/);
    assert.deepEqual(published(root), []);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("dry run packages all tarballs without registry queries or publishing", () => {
  const root = fixture();
  try {
    const result = run(root, "conflict", "1.2.3", "--dry-run");
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /preflight passed for 7 npm tarballs/);
    assert.deepEqual(published(root), []);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("prepared mode publishes the exact preflighted tarballs", () => {
  const root = fixture();
  try {
    const dry = run(root, "fresh", "1.2.3", "--dry-run");
    assert.equal(dry.status, 0, dry.stderr);
    const metadata = join(root, "npm-stage", "packages.json");
    const before = readFileSync(metadata, "utf8");
    const result = run(root, "fresh", "1.2.3", "--prepared");
    assert.equal(result.status, 0, result.stderr);
    assert.equal(readFileSync(metadata, "utf8"), before);
    assert.equal(published(root).length, 7);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("accepts npm view's flat dist metadata keys", () => {
  const root = fixture();
  try {
    const result = run(root, "flat", "1.2.3");
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(published(root), []);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("missing and duplicate binaries fail in preflight", () => {
  for (const mutation of ["missing", "duplicate"]) {
    const root = fixture();
    try {
      const manifestPath = join(root, "dist", "artifacts.json");
      const artifacts = JSON.parse(readFileSync(manifestPath, "utf8"));
      if (mutation === "missing") artifacts.shift(); else artifacts.push({ ...artifacts[0] });
      writeFileSync(manifestPath, JSON.stringify(artifacts));
      const result = run(root, "fresh", "1.2.3");
      assert.equal(result.status, 1);
      assert.match(result.stderr, /expected one git-chunks Binary/);
      assert.deepEqual(published(root), []);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  }
});

test("rejects invalid SemVer before packaging", () => {
  const root = fixture();
  try {
    const result = run(root, "fresh", "01.2.3");
    assert.equal(result.status, 2);
    assert.match(result.stderr, /SemVer/);
    assert.deepEqual(published(root), []);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
