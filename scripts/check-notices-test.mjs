import { mkdtempSync, mkdirSync, copyFileSync, appendFileSync, rmSync } from "node:fs";
import { execFileSync, spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join } from "node:path";
import assert from "node:assert/strict";

const root = mkdtempSync(join(tmpdir(), "hscope-notices-test-"));
try {
  mkdirSync(join(root, "web")); mkdirSync(join(root, "scripts"));
  for (const file of ["go.sum", "web/package-lock.json", "THIRD_PARTY_NOTICES.md", "scripts/check-third-party-notices.sh"]) copyFileSync(file, join(root, file));
  const script = join(root, "scripts/check-third-party-notices.sh");
  execFileSync("sh", [script]);
  for (const file of ["go.sum", "web/package-lock.json"]) {
    appendFileSync(join(root, file), "\nsynthetic-stale-lock\n");
    const result = spawnSync("sh", [script], { encoding: "utf8" });
    assert.equal(result.status, 1);
    assert.ok(result.stderr.includes(`${file} changed`));
    copyFileSync(file, join(root, file));
  }
  console.log("Both independent stale dependency hashes rejected PASS");
} finally { rmSync(root, { recursive: true, force: true }); }
