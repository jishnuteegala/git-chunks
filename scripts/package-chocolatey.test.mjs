import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { tmpdir } from "node:os";
import { delimiter, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const root = resolve(import.meta.dirname, "..");
const powershell = process.platform === "win32" ? "powershell.exe" : "pwsh";

test("Chocolatey package uses canonical Windows archives and checksums", { skip: process.platform !== "win32" }, () => {
  const work = mkdtempSync(join(tmpdir(), "git-chunks-choco-"));
  const dist = join(work, "dist");
  const bin = join(work, "bin");
  const temp = join(work, "temp");
  mkdirSync(dist);
  mkdirSync(bin);
  mkdirSync(temp);
  const amd64Archive = Buffer.from("amd64 archive");
  const arm64Archive = Buffer.from("arm64 archive");
  const amd64 = createHash("sha256").update(amd64Archive).digest("hex");
  const arm64 = createHash("sha256").update(arm64Archive).digest("hex");
  writeFileSync(join(dist, "git-chunks_1.2.3_windows_amd64.zip"), amd64Archive);
  writeFileSync(join(dist, "git-chunks_1.2.3_windows_arm64.zip"), arm64Archive);
  writeFileSync(join(dist, "checksums.txt"), [
    `${amd64}  git-chunks_1.2.3_windows_amd64.zip`,
    `${arm64}  git-chunks_1.2.3_windows_arm64.zip`,
  ].join("\n"));
  writeFileSync(join(bin, "choco.cmd"), "@echo off\r\ntype nul > %4\\git-chunks.1.2.3.nupkg\r\n");

  const result = spawnSync(powershell, ["-NoProfile", "-File", join(root, "scripts", "package-chocolatey.ps1"), "-Version", "1.2.3"], {
    encoding: "utf8",
    env: {
      ...process.env,
      Path: `${bin}${delimiter}${process.env.Path}`,
      PUBLISH_CHOCOLATEY_ROOT: work,
      PUBLISH_CHOCOLATEY_CLI: join(bin, "choco.cmd"),
      TEMP: temp,
    },
  });

  try {
    assert.equal(result.status, 0, result.stderr || result.stdout);
    const stage = join(temp, "git-chunks-chocolatey-1.2.3");
    const nuspec = readFileSync(join(stage, "git-chunks.nuspec"), "utf8");
    const install = readFileSync(join(stage, "tools", "chocolateyinstall.ps1"), "utf8");
    assert.match(nuspec, /<version>1\.2\.3<\/version>/);
    assert.match(install, /git-chunks_1\.2\.3_windows_amd64\.zip/);
    assert.match(install, /git-chunks_1\.2\.3_windows_arm64\.zip/);
    assert.match(install, new RegExp(amd64));
    assert.match(install, new RegExp(arm64));
    assert.match(install, /RuntimeInformation.*OSArchitecture.*Arm64/);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});
