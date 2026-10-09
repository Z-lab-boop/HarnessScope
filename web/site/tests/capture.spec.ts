import { test, expect } from "@playwright/test";
import { execFileSync } from "node:child_process";

for (const [path, capture] of [["/", "hero"], ["/explore.html", "explore"], ["/docs.html", "docs"]] as const) {
  test(`${capture} capture surface is stable`, async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(path);
    await expect(page.locator(`[data-capture="${capture}"]`)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("README public-site assets satisfy repository policy", () => {
  execFileSync(process.execPath, ["scripts/check-site-assets.mjs"], { cwd: new URL("../../../", import.meta.url), stdio: "pipe" });
});
