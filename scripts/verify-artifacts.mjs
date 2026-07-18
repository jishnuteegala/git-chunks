#!/usr/bin/env node
import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { basename, isAbsolute, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const manifestPath = resolve(root, process.argv[2] || "dist/artifacts.json");
const version = process.argv[3] || "0.0.0-SNAPSHOT";
const artifacts = JSON.parse(readFileSync(manifestPath, "utf8"));
const releaseTypes = new Set(["Archive", "Linux Package", "Checksum"]);
const releaseArtifacts = artifacts.filter((artifact) => releaseTypes.has(artifact.type));
const readme = readFileSync(resolve(root, "README.md"), "utf8");
const documented = [...readme.matchAll(/(?:git-chunks_\$\{VERSION\}_[\w.]+|checksums\.txt)/g)]
  .map(([name]) => name.replace("${VERSION}", version));
const expected = [...new Set(documented)];
const errors = [];

function artifactPath(artifact) {
  return isAbsolute(artifact.path) ? artifact.path : resolve(root, artifact.path);
}

const byName = new Map();
for (const artifact of releaseArtifacts) {
  const name = basename(artifact.path || "");
  if (!name || !artifact.type) errors.push(`artifact has incomplete type/path metadata: ${JSON.stringify(artifact)}`);
  if (byName.has(name)) errors.push(`duplicate release artifact name: ${name}`);
  byName.set(name, artifact);
  if (!existsSync(artifactPath(artifact))) errors.push(`missing artifact file: ${artifact.path}`);
}

for (const name of expected) {
  if (!byName.has(name)) errors.push(`missing documented artifact: ${name}`);
}
for (const name of byName.keys()) {
  if (!expected.includes(name)) errors.push(`undocumented release artifact: ${name}`);
}

for (const artifact of releaseArtifacts) {
  const name = basename(artifact.path);
  if (artifact.type === "Archive") {
    const match = name.match(/^git-chunks_.+_(linux|darwin|windows)_(amd64|arm64)\.(?:tar\.gz|zip)$/);
    if (!match || artifact.goos !== match[1] || artifact.goarch !== match[2]) {
      errors.push(`archive metadata does not match filename: ${name}`);
    }
  }
  if (artifact.type === "Linux Package") {
    const match = name.match(/^git-chunks_.+_linux_(amd64|arm64)\.(?:deb|rpm|apk|pkg\.tar\.zst)$/);
    if (!match || artifact.goos !== "linux" || artifact.goarch !== match[1]) {
      errors.push(`package metadata does not match filename: ${name}`);
    }
  }
}

const checksumArtifact = byName.get("checksums.txt");
if (checksumArtifact && existsSync(artifactPath(checksumArtifact))) {
  const checksums = new Map(readFileSync(artifactPath(checksumArtifact), "utf8").trim().split(/\r?\n/).map((line) => {
    const match = line.match(/^([a-f\d]{64})\s{2}(.+)$/i);
    return match ? [match[2], match[1].toLowerCase()] : [line, ""];
  }));
  for (const artifact of releaseArtifacts.filter(({ type }) => type !== "Checksum")) {
    const name = basename(artifact.path);
    const actual = createHash("sha256").update(readFileSync(artifactPath(artifact))).digest("hex");
    if (checksums.get(name) !== actual) errors.push(`missing or invalid checksum: ${name}`);
  }
  for (const name of checksums.keys()) {
    if (!byName.has(name)) errors.push(`checksum references unknown release artifact: ${name}`);
  }
}

if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}

console.log(`verified ${expected.length} documented, typed, checksummed release artifacts`);
