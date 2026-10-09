import assert from "node:assert/strict";
import { readFileSync, statSync } from "node:fs";
import { execFileSync } from "node:child_process";

const assets = ["docs/assets/site-hero.png", "docs/assets/site-explore.png", "docs/assets/site-docs.png"];
const signature = "89504e470d0a1a0a";
for (const file of assets) {
  let stat;
  try { stat = statSync(file); } catch { throw new Error(`${file}: missing`); }
  assert.equal(stat.isFile(), true, `${file}: not a regular file`);
  assert.ok(stat.size <= 2 * 1024 * 1024, `${file}: exceeds 2 MiB`);
  const bytes = readFileSync(file);
  assert.equal(bytes.subarray(0, 8).toString("hex"), signature, `${file}: invalid PNG signature`);
  assert.ok(bytes.readUInt32BE(16) >= 320 && bytes.readUInt32BE(20) >= 180, `${file}: dimensions are too small`);
}

const links = [
  "https://z-lab-boop.github.io/HarnessScope/",
  "https://z-lab-boop.github.io/HarnessScope/explore.html",
  "https://z-lab-boop.github.io/HarnessScope/docs.html",
];
for (const file of ["README.md", "README.zh-CN.md"]) {
  const markdown = readFileSync(file, "utf8");
  assert.match(markdown, /!\[([^\]]\S(?:[^\]]*\S)?)\]\(docs\/assets\/site-hero\.png\)/, `${file}: missing site hero with nonempty alt text`);
  for (const link of links) assert.ok(markdown.includes(`](${link})`), `${file}: missing ${link}`);
}

execFileSync("git", ["diff", "--check"], { stdio: "inherit" });
console.log("Public site PNG and bilingual README link policy PASS");
