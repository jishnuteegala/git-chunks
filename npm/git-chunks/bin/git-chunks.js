#!/usr/bin/env node
// Thin launcher: resolves the platform-specific binary package and execs it.
const { spawnSync } = require("child_process");
const path = require("path");

const packages = {
  "linux-x64": "git-chunks-linux-x64",
  "linux-arm64": "git-chunks-linux-arm64",
  "darwin-x64": "git-chunks-darwin-x64",
  "darwin-arm64": "git-chunks-darwin-arm64",
  "win32-x64": "git-chunks-windows-x64",
  "win32-arm64": "git-chunks-windows-arm64",
};

const key = `${process.platform}-${process.arch}`;
const pkg = packages[key];
if (!pkg) {
  console.error(`git-chunks: unsupported platform ${key}`);
  process.exit(1);
}

const binaryName = process.platform === "win32" ? "git-chunks.exe" : "git-chunks";
let binary;
try {
  binary = path.join(path.dirname(require.resolve(`${pkg}/package.json`)), "bin", binaryName);
} catch {
  console.error(
    `git-chunks: platform package ${pkg} is not installed.\n` +
      "This can happen if optional dependencies are disabled. " +
      "Reinstall without --no-optional, or download a binary from " +
      "https://github.com/jishnuteegala/git-chunks/releases",
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
