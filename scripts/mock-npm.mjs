#!/usr/bin/env node
import { appendFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join, resolve } from "node:path";

const args = process.argv.slice(2);
const command = args.shift();
const root = resolve(process.env.PUBLISH_NPM_ROOT);
const state = process.env.MOCK_NPM_STATE || "fresh";
const log = join(root, "npm-calls.log");
const publishedLog = join(root, "npm-published.log");

function metadata(name, version) {
  const pkg = JSON.parse(readFileSync(join(root, "npm-stage", name, "package.json"), "utf8"));
  return {
    ...pkg,
    dist: {
      integrity: `sha512-${Buffer.from(name).toString("base64")}`,
      shasum: Buffer.from(name).toString("hex").slice(0, 40).padEnd(40, "0"),
    },
  };
}

if (command === "pack") {
  const packageDir = resolve(args[0]);
  const destination = resolve(args[args.indexOf("--pack-destination") + 1]);
  const pkg = JSON.parse(readFileSync(join(packageDir, "package.json"), "utf8"));
  const binary = pkg.name === "git-chunks"
    ? "bin/git-chunks.js"
    : `bin/${pkg.os[0] === "win32" ? "git-chunks.exe" : "git-chunks"}`;
  const filename = `${pkg.name}-${pkg.version}.tgz`;
  mkdirSync(destination, { recursive: true });
  writeFileSync(join(destination, filename), pkg.name);
  process.stdout.write(JSON.stringify([{
    filename,
    integrity: `sha512-${Buffer.from(pkg.name).toString("base64")}`,
    shasum: Buffer.from(pkg.name).toString("hex").slice(0, 40).padEnd(40, "0"),
    files: [{ path: "package.json" }, { path: binary }],
  }]));
  process.exit(0);
}

if (command === "view") {
  const [spec] = args;
  const split = spec.lastIndexOf("@");
  const name = spec.slice(0, split);
  const version = spec.slice(split + 1);
  const existing = ["complete", "flat"].includes(state)
    || state === "conflict"
    || (state === "partial" && ["git-chunks-linux-x64", "git-chunks-linux-arm64"].includes(name))
    || (() => {
      try {
        return readFileSync(publishedLog, "utf8").split("\n").includes(name);
      } catch {
        return false;
      }
    })();
  if (!existing) {
    process.stdout.write(JSON.stringify({ error: { code: "E404" } }));
    process.exit(1);
  }
  const value = metadata(name, version);
  if (state === "conflict" && name === "git-chunks") value.dist.integrity = "sha512-conflict";
  if (state === "flat") {
    value["dist.integrity"] = value.dist.integrity;
    value["dist.shasum"] = value.dist.shasum;
    delete value.dist;
  }
  process.stdout.write(JSON.stringify(value));
  process.exit(0);
}

if (command === "publish") {
  appendFileSync(log, `${basename(args[0])}\n`);
  appendFileSync(publishedLog, `${basename(args[0]).replace(/-\d+\.\d+\.\d+(?:-[^.]+)?\.tgz$/, "")}\n`);
  process.exit(0);
}

process.stderr.write(`unexpected mock npm command: ${command}\n`);
process.exit(2);
