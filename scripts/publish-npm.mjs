#!/usr/bin/env node
import { chmodSync, copyFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, dirname, isAbsolute, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const platforms = [
  { key: "linux-x64", goos: "linux", goarch: "amd64", os: "linux", cpu: "x64" },
  { key: "linux-arm64", goos: "linux", goarch: "arm64", os: "linux", cpu: "arm64" },
  { key: "darwin-x64", goos: "darwin", goarch: "amd64", os: "darwin", cpu: "x64" },
  { key: "darwin-arm64", goos: "darwin", goarch: "arm64", os: "darwin", cpu: "arm64" },
  { key: "windows-x64", goos: "windows", goarch: "amd64", os: "win32", cpu: "x64" },
  { key: "windows-arm64", goos: "windows", goarch: "arm64", os: "win32", cpu: "arm64" },
];

const semver = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;
const version = process.argv[2];
const dryRun = process.argv[3] === "--dry-run";
if (!version || !semver.test(version) || (process.argv[3] && !dryRun) || process.argv.length > 4) {
  console.error("usage: publish-npm.sh <SemVer version> [--dry-run]");
  process.exit(2);
}

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const root = resolve(process.env.PUBLISH_NPM_ROOT || scriptRoot);
const dist = join(root, "dist");
const stage = join(root, "npm-stage");
const tarballs = join(stage, "tarballs");
const npm = process.env.PUBLISH_NPM_CLI || "npm";
const npmPrefix = process.env.PUBLISH_NPM_CLI_SCRIPT ? [process.env.PUBLISH_NPM_CLI_SCRIPT] : [];

function runNpm(args, cwd) {
  return spawnSync(npm, [...npmPrefix, ...args], {
    cwd,
    encoding: "utf8",
    env: process.env,
    shell: process.platform === "win32" && npmPrefix.length === 0,
  });
}

function npmJson(args, cwd) {
  const result = runNpm(args, cwd);
  if (result.status !== 0) {
    throw new Error((result.stderr || result.stdout || `npm ${args[0]} failed`).trim());
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    throw new Error(`npm ${args[0]} returned invalid JSON`);
  }
}

function artifactPath(artifact) {
  return isAbsolute(artifact.path) ? artifact.path : resolve(root, artifact.path);
}

function expectedBinary(goos) {
  return goos === "windows" ? "git-chunks.exe" : "git-chunks";
}

function platformPackage(name, platform) {
  return {
    name,
    version,
    description: `git-chunks binary for ${platform.key}`,
    repository: "github:jishnuteegala/git-chunks",
    license: "MIT",
    os: [platform.os],
    cpu: [platform.cpu],
  };
}

function same(value, expected) {
  return JSON.stringify(value) === JSON.stringify(expected);
}

function validateExisting(actual, expected, packed) {
  const checks = [
    ["name", actual.name, expected.name],
    ["version", actual.version, expected.version],
    ["description", actual.description, expected.description],
    ["license", actual.license, expected.license],
    ["os", actual.os, expected.os],
    ["cpu", actual.cpu, expected.cpu],
    ["bin", actual.bin, expected.bin],
    ["optionalDependencies", actual.optionalDependencies, expected.optionalDependencies],
    ["dist.integrity", actual.dist?.integrity, packed.integrity],
    ["dist.shasum", actual.dist?.shasum, packed.shasum],
  ];
  const mismatch = checks.find(([, actualValue, expectedValue]) => !same(actualValue, expectedValue));
  if (mismatch) throw new Error(`${expected.name}@${version} exists with unexpected ${mismatch[0]}`);
}

function queryExisting(pkg) {
  const result = runNpm([
    "view",
    `${pkg.json.name}@${version}`,
    "name", "version", "description", "license", "os", "cpu", "bin", "optionalDependencies",
    "dist.integrity", "dist.shasum", "--json",
  ], root);
  if (result.status === 0) {
    let actual;
    try {
      actual = JSON.parse(result.stdout);
    } catch {
      throw new Error(`npm view returned invalid JSON for ${pkg.json.name}@${version}`);
    }
    validateExisting(actual, pkg.json, pkg.packed);
    return true;
  }
  let code;
  try {
    code = JSON.parse(result.stdout).error?.code;
  } catch {
    // npm may write errors to stderr depending on its version.
  }
  if (code === "E404" || /E404|404 Not Found/.test(`${result.stdout}\n${result.stderr}`)) return false;
  throw new Error((result.stderr || result.stdout || `npm view failed for ${pkg.json.name}`).trim());
}

function createPackages() {
  const manifestPath = join(dist, "artifacts.json");
  if (!existsSync(manifestPath)) throw new Error(`missing ${manifestPath}`);
  const artifacts = JSON.parse(readFileSync(manifestPath, "utf8"));
  rmSync(stage, { recursive: true, force: true });
  mkdirSync(tarballs, { recursive: true });

  const packages = platforms.map((platform) => {
    const matches = artifacts.filter((artifact) =>
      artifact.type === "Binary" && artifact.extra?.ID === "git-chunks"
      && artifact.goos === platform.goos && artifact.goarch === platform.goarch);
    if (matches.length !== 1) {
      throw new Error(`expected one git-chunks Binary for ${platform.goos}/${platform.goarch}, found ${matches.length}`);
    }
    const binary = expectedBinary(platform.goos);
    const source = artifactPath(matches[0]);
    if (basename(source) !== binary) throw new Error(`${platform.key} artifact must be named ${binary}`);
    if (!existsSync(source)) throw new Error(`missing artifact ${source}`);

    const json = platformPackage(`git-chunks-${platform.key}`, platform);
    const dir = join(stage, json.name);
    mkdirSync(join(dir, "bin"), { recursive: true });
    const destination = join(dir, "bin", binary);
    copyFileSync(source, destination);
    chmodSync(destination, 0o755);
    writeFileSync(join(dir, "package.json"), `${JSON.stringify(json, null, 2)}\n`);
    return { dir, json, binary };
  });

  const sourceMain = JSON.parse(readFileSync(join(root, "npm", "git-chunks", "package.json"), "utf8"));
  sourceMain.version = version;
  for (const dependency of Object.keys(sourceMain.optionalDependencies)) {
    sourceMain.optionalDependencies[dependency] = version;
  }
  const mainDir = join(stage, "git-chunks");
  mkdirSync(join(mainDir, "bin"), { recursive: true });
  copyFileSync(join(root, "npm", "git-chunks", "bin", "git-chunks.js"), join(mainDir, "bin", "git-chunks.js"));
  copyFileSync(join(root, "README.md"), join(mainDir, "README.md"));
  writeFileSync(join(mainDir, "package.json"), `${JSON.stringify(sourceMain, null, 2)}\n`);
  packages.push({ dir: mainDir, json: sourceMain, binary: "bin/git-chunks.js" });
  return packages;
}

function preflight(packages) {
  for (const pkg of packages) {
    const packed = npmJson(["pack", pkg.dir, "--pack-destination", tarballs, "--json"], root)[0];
    if (!packed?.filename || !packed.integrity || !packed.shasum) {
      throw new Error(`npm pack returned incomplete metadata for ${pkg.json.name}`);
    }
    const files = packed.files?.map((file) => file.path) || [];
    const expectedFile = pkg.binary.startsWith("bin/") ? pkg.binary : `bin/${pkg.binary}`;
    if (!files.includes("package.json") || !files.includes(expectedFile)) {
      throw new Error(`${pkg.json.name} tarball is missing package.json or ${expectedFile}`);
    }
    pkg.packed = packed;
    pkg.tarball = join(tarballs, packed.filename);
    if (!existsSync(pkg.tarball)) throw new Error(`missing packed tarball ${pkg.tarball}`);
  }
}

function publish(packages) {
  const existing = new Map(packages.map((pkg) => [pkg.json.name, queryExisting(pkg)]));
  for (const pkg of packages) {
    if (existing.get(pkg.json.name)) {
      console.log(`verified existing ${pkg.json.name}@${version}`);
      continue;
    }
    const result = runNpm(["publish", pkg.tarball, "--access", "public"], root);
    if (result.status !== 0) throw new Error((result.stderr || result.stdout || `npm publish failed`).trim());
    if (!queryExisting(pkg)) throw new Error(`${pkg.json.name}@${version} was not visible after publishing`);
    console.log(`published ${pkg.json.name}@${version}`);
  }
}

try {
  const packages = createPackages();
  preflight(packages);
  console.log(`preflight passed for ${packages.length} npm tarballs`);
  if (!dryRun) publish(packages); else console.log("dry run: nothing published");
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
}
