import assert from "node:assert/strict";
import { readFileSync, statSync } from "node:fs";

const assets = ["docs/assets/site-hero.png", "docs/assets/site-explore.png", "docs/assets/site-docs.png"];
const signature = "89504e470d0a1a0a";
for (const file of assets) {
  let stat;
  try { stat = statSync(file); } catch { throw new Error(`${file}: missing`); }
  assert.equal(stat.isFile(), true, `${file}: not a regular file`);
  assert.ok(stat.size <= 2 * 1024 * 1024, `${file}: exceeds 2 MiB`);
  const bytes = readFileSync(file);
  assert.ok(bytes.length >= 33 && bytes.subarray(0, 8).toString("hex") === signature && bytes.toString("ascii", 12, 16) === "IHDR", `${file}: invalid PNG signature or header`);
  const width = bytes.readUInt32BE(16);
  const height = bytes.readUInt32BE(20);
  assert.ok(width >= 320 && width <= 1600 && height >= 180 && height <= 2400, `${file}: dimensions ${width}×${height} outside GitHub preview bounds`);
}

const links = [
  "https://z-lab-boop.github.io/HarnessScope/",
  "https://z-lab-boop.github.io/HarnessScope/explore.html",
  "https://z-lab-boop.github.io/HarnessScope/docs.html",
  "https://github.com/Z-lab-boop/HarnessScope/releases",
];
for (const file of ["README.md", "README.zh-CN.md"]) {
  const markdown = readFileSync(file, "utf8");
  assert.match(markdown, /^!\[[^\]\n]*\S[^\]\n]*\]\(docs\/assets\/site-hero\.png\)\s+\# HarnessScope\s+/, `${file}: opening must lead with the site hero and HarnessScope heading`);
  for (const asset of assets) {
    const filename = asset.split("/").at(-1);
    const escaped = asset.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const markdownAlt = new RegExp(`!\\[([^\\]\\n]*)\\]\\(${escaped}\\)`).exec(markdown)?.[1];
    const htmlTag = new RegExp(`<img\\s[^>]*src="${escaped}"[^>]*>`).exec(markdown)?.[0];
    const htmlAlt = htmlTag && /\balt="([^"]*)"/.exec(htmlTag)?.[1];
    assert.ok((markdownAlt ?? htmlAlt)?.trim(), `${file}: ${filename} requires nonempty alt text`);
  }
  for (const link of links) assert.ok(markdown.includes(`](${link})`), `${file}: missing ${link}`);
}

const releaseFiles = readFileSync("scripts/release-files.txt", "utf8").trim().split("\n");
for (const asset of assets) assert.equal(releaseFiles.filter(file => file === asset).length, 1, `scripts/release-files.txt: ${asset} must appear exactly once`);
console.log("Public site PNG dimensions, bilingual README links/alt text and release asset policy PASS");
