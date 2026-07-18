#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const manifest = JSON.parse(readFileSync(process.argv[2] || "dist/artifacts.json", "utf8"));
const types = new Set(["Archive", "Linux Package", "Checksum"]);
for (const artifact of manifest.filter(({ type }) => types.has(type))) {
  console.log(resolve(artifact.path));
}
