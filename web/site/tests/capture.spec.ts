import { test, expect } from "@playwright/test";
import { execFileSync, spawnSync } from "node:child_process";
import { cpSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const repository = fileURLToPath(new URL("../../../", import.meta.url));

test("capture server proves child ownership and rejects foreign listeners", () => {
  execFileSync(process.execPath, ["--test", "site/capture-server.test.mjs"], { cwd: join(repository, "web"), stdio: "pipe", timeout: 15000 });
});

for (const [path, capture] of [["/", "hero"], ["/explore.html", "explore"], ["/docs.html", "docs"]] as const) {
  test(`${capture} capture surface is stable`, async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(path);
    const surface = page.locator(`[data-capture="${capture}"]`);
    await expect(surface).toHaveCount(1);
    await expect(surface).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(Array.from(document.images).map(image => image.decode()));
    });
    expect(await surface.locator("img").evaluateAll(images => images.every(image => image instanceof HTMLImageElement && image.naturalWidth > 0 && image.naturalHeight > 0))).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("README public-site assets satisfy repository policy", () => {
  execFileSync(process.execPath, ["scripts/check-site-assets.mjs"], { cwd: new URL("../../../", import.meta.url), stdio: "pipe" });
});

test("README asset checks identify broken previews, dimensions, links and archive members", () => {
  const root = mkdtempSync(join(tmpdir(), "hscope-site-policy-"));
  try {
    mkdirSync(join(root, "docs/assets"), { recursive: true });
    mkdirSync(join(root, "scripts"));
    const files = ["README.md", "README.zh-CN.md", "scripts/release-files.txt", "docs/assets/site-hero.png", "docs/assets/site-explore.png", "docs/assets/site-docs.png"];
    for (const file of files) cpSync(join(repository, file), join(root, file));
    const check = () => spawnSync(process.execPath, [join(repository, "scripts/check-site-assets.mjs")], { cwd: root, encoding: "utf8" });
    expect(check().status).toBe(0);
    const reject = (file: string, bytes: string | Buffer, condition: string) => {
      const original = readFileSync(join(root, file));
      writeFileSync(join(root, file), bytes);
      const result = check();
      expect(result.status, condition).not.toBe(0);
      expect(result.stderr).toContain(condition);
      writeFileSync(join(root, file), original);
    };
    for (const file of ["README.md", "README.zh-CN.md"]) {
      const original = readFileSync(join(root, file), "utf8");
      reject(file, original.replace(/alt="HarnessScope [^"]*"(?=><\/a><br><strong>Explore)/, 'alt=""').replace(/!\[HarnessScope [^\]]*\]\(docs\/assets\/site-explore\.png\)/, "![](docs/assets/site-explore.png)"), `${file}: site-explore.png requires nonempty alt text`);
      reject(file, original.split("https://z-lab-boop.github.io/HarnessScope/docs.html").join("https://example.invalid/docs.html"), `${file}: missing https://z-lab-boop.github.io/HarnessScope/docs.html`);
    }
    const imageFile = "docs/assets/site-docs.png";
    reject(imageFile, Buffer.alloc(2 * 1024 * 1024 + 1), `${imageFile}: exceeds 2 MiB`);
    reject(imageFile, Buffer.from("not a PNG"), `${imageFile}: invalid PNG`);
    const small = Buffer.from(readFileSync(join(root, imageFile)));
    small.writeUInt32BE(1, 16);
    reject(imageFile, small, `${imageFile}: dimensions`);
    reject("scripts/release-files.txt", readFileSync(join(root, "scripts/release-files.txt"), "utf8").replace("docs/assets/site-docs.png\n", ""), "scripts/release-files.txt: docs/assets/site-docs.png must appear exactly once");
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
