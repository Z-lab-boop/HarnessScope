import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { execFileSync, spawnSync } from "node:child_process";

const repository = process.cwd();
const roots = [];
const temp = () => { const root = mkdtempSync(join(tmpdir(), "hscope-release-test-")); roots.push(root); return root; };
process.on("exit", () => { for (const root of roots) rmSync(root, { recursive: true, force: true }); });
const put = (path, value) => { mkdirSync(dirname(path), { recursive: true }); writeFileSync(path, value); };

test("Linux missing baselines are OPEN in ordinary CI and blocking only in strict release", () => {
  const root = temp();
  const script = join(root, "scripts/check-visual-baselines.sh");
  put(script, readFileSync("scripts/check-visual-baselines.sh"));
  const output = join(root, "output"), summary = join(root, "summary");
  const run = (strict) => spawnSync("sh", [script], { encoding: "utf8", env: { ...process.env, GITHUB_OUTPUT: output, GITHUB_STEP_SUMMARY: summary, HARNESSSCOPE_STRICT_RELEASE: String(strict) } });
  assert.equal(run(false).status, 0);
  assert.match(readFileSync(summary, "utf8"), /OPEN/);
  assert.match(readFileSync(output, "utf8"), /available=false/);
  assert.equal(run(true).status, 1);
  for (const file of ["web/tests/__snapshots__/report-linux.png", "web/dashboard/tests/__snapshots__/dashboard-linux.png"]) put(join(root, file), "synthetic test placeholder, not a baseline");
  assert.equal(run(true).status, 1, "Graph is required too");
  put(join(root, "web/dashboard/tests/__snapshots__/graph-linux.png"), "synthetic test placeholder, not a baseline");
  assert.equal(run(true).status, 0);
  assert.match(readFileSync(output, "utf8"), /available=true/);
  const ci = readFileSync(".github/workflows/ci.yml", "utf8"), release = readFileSync(".github/workflows/release.yml", "utf8");
  assert.match(ci, /strict_release:[\s\S]*?default: false/);
  assert.match(ci, /if: steps\.baselines\.outputs\.available == 'true'/);
  assert.match(release, /gates:[\s\S]*?strict_release: true/);
  assert.match(release, /build:\s+needs: gates/);
  assert.ok(!release.includes("cp -R docs"));
  assert.match(release, /build-release\.sh "\$VERSION"/);
});

test("archive checker rejects internal files, personal paths and symlinks after exact allowlist validation", () => {
  const root = temp(), payload = join(root, "payload"), archive = join(root, "artifact.tar.gz");
  const members = readFileSync("scripts/release-files.txt", "utf8").trim().split("\n");
  for (const member of members) put(join(payload, member), "synthetic public content\n");
  const packageFiles = (extra = []) => execFileSync("tar", ["-C", payload, "-czf", archive, ...members, ...extra]);
  const check = () => spawnSync("sh", [resolve("scripts/check-release-archive.sh"), archive], { encoding: "utf8" });
  packageFiles(); assert.equal(check().status, 0);
  put(join(payload, "docs/internal-plan.md"), "internal planning");
  packageFiles(["docs/internal-plan.md"]); assert.equal(check().status, 1);
  put(join(payload, "README.md"), "synthetic private path /Users/release-private/project\n");
  packageFiles(); assert.equal(check().status, 1);
  put(join(payload, "README.md"), "synthetic public content\n");
  rmSync(join(payload, "docs/architecture.md"));
  execFileSync("ln", ["-s", "../README.md", join(payload, "docs/architecture.md")]);
  packageFiles(); assert.equal(check().status, 1);
});

test("invalid release version and target are refused before producing an output", () => {
  const root = temp();
  for (const args of [["v../unsafe", join(root, "bad-version")], ["v9.8.7\nbad", join(root, "multiline-version")], ["v9.8.7", join(root, "bad-target"), "windows-amd64"]]) {
    const result = spawnSync("sh", ["scripts/build-release.sh", ...args], { cwd: repository, encoding: "utf8" });
    assert.equal(result.status, 2);
    assert.equal(existsSync(args[1]), false);
  }
});
