import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync } from "node:fs";
import assert from "node:assert/strict";

// Strict JSON parsing rejects comments, trailing commas, and truncated schemas.
for (const file of readdirSync("schemas").filter((name) => name.endsWith(".json"))) {
  const schema = JSON.parse(readFileSync(`schemas/${file}`, "utf8"));
  assert.equal(schema.type, "object", file);
  assert.equal(schema.additionalProperties, false, file);
  assert.ok(schema.$schema && schema.$id, file);
}
const allowed = /^(fixtures\/|demo\/conflicted-workspace\/|demo\/run\.sh$|internal\/.+_test\.go$|scripts\/(smoke-release\.sh|check-release-archive\.sh|check-repository\.mjs)$|docs\/|README|CONTRIBUTING\.md$|SECURITY\.md$|THIRD_PARTY_NOTICES\.md$|\.github\/workflows\/ci\.yml$)/;
const files = execFileSync("git", ["ls-files", "-z"], { encoding: "utf8" }).split("\0").filter(Boolean);
for (const file of files) {
  if (file.startsWith(".superpowers/")) continue; // local implementation notes, not shipped artifacts
  const bytes = readFileSync(file);
  if (file === "internal/export/bundle.go") {
    // The runtime exporter deliberately rejects this exact synthetic marker.
    const lines = bytes.toString("utf8").split("\n").filter((line) => line.includes("HARNESSSCOPE-CANARY"));
    assert.equal(lines.length, 1);
    assert.equal(lines[0].trim(), 'if strings.Contains(value, "HARNESSSCOPE-CANARY") || strings.Contains(value, "HARNESSCOPE-CANARY") || homePathPattern.MatchString(value) {');
    continue;
  }
  if (bytes.includes("HARNESSSCOPE-CANARY")) assert.ok(allowed.test(file), `canary outside synthetic input/test/policy: ${file}`);
}
execFileSync(process.execPath, ["scripts/check-site-assets.mjs"], { stdio: "inherit" });
console.log("Repository canary policy and five strict JSON schema parses PASS");
