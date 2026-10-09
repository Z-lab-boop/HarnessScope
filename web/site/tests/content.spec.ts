import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { content, english, type ContentKey } from "../src/content.js";

test("translation dictionaries have exact nonempty bilingual keys", () => {
  expect(Object.keys(content["zh-CN"]).sort()).toEqual(Object.keys(english).sort());
  for (const locale of ["en", "zh-CN"] as const) {
    for (const value of Object.values(content[locale])) expect(value.trim()).not.toBe("");
  }
});

test("every declared translation renders in both languages", async ({ page }) => {
  for (const path of ["/", "/explore.html", "/docs.html"]) {
    await page.goto(path);
    if (path === "/explore.html") await expect(page.locator("[data-explore-workbench]")).toBeVisible();
    expect(await page.locator("[data-i18n]").count()).toBeGreaterThan(0);
    expect(await page.locator("[data-i18n]").evaluateAll(elements => elements.some(element => /undefined|missing/i.test(element.textContent ?? "")))).toBe(false);
    const pageKey = path === "/" ? "home" : path === "/docs.html" ? "docs" : "explore";
    for (const locale of ["en", "zh-CN"] as const) {
      if (locale === "zh-CN") await page.getByRole("button", { name: "切换到中文" }).click();
      await expect(page.locator("html")).toHaveAttribute("lang", locale);
      for (const [attribute, target] of [["data-i18n", "textContent"], ["data-i18n-aria", "aria-label"], ["data-i18n-alt", "alt"]] as const) {
        const entries = await page.locator(`[${attribute}]`).evaluateAll((elements, args) => elements.map(element => ({
          key: element.getAttribute(args.attribute)!,
          value: args.target === "textContent" ? element.textContent : element.getAttribute(args.target),
        })), { attribute, target });
        for (const entry of entries) {
          expect(entry.key in english, `${path}: unknown ${attribute} key ${entry.key}`).toBe(true);
          expect(entry.value, `${path}: untranslated ${entry.key} in ${locale}`).toBe(content[locale][entry.key as ContentKey]);
        }
      }
      await expect(page).toHaveTitle(`${content[locale][`${pageKey}.title`]} — HarnessScope`);
      await expect(page.locator('meta[name="description"]')).toHaveAttribute("content", content[locale][`${pageKey}.body`]);
      if (pageKey === "docs") await expect(page.locator(".terminal-bar")).toContainText(content[locale]["docs.sourceLabel"]);
      expect((await page.locator("main").innerText()).trim().length).toBeGreaterThan(200);
    }
    await page.getByRole("button", { name: "Switch to English" }).click();
  }
});

test("Docs states the truthful release boundary", async ({ page }) => {
  await page.goto("/docs.html");
  await expect(page.getByRole("heading", { name: "Build from source" })).toBeVisible();
  await expect(page.getByText("Download v0.2", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "GitHub Releases" })).toHaveAttribute("href", /releases/);
  await expect(page.getByText("No tagged binary release is published yet.", { exact: true })).toBeVisible();
  await expect(page.locator(".terminal-panel code")).toContainText("go build -trimpath -o bin/hscope ./cmd/hscope");
  await expect(page.locator(".terminal-panel code")).toContainText("./bin/hscope serve . --port 0 --open");
  for (const [key, path] of [["docs.architecture", "docs/architecture.md"], ["docs.security", "SECURITY.md"], ["docs.contributing", "CONTRIBUTING.md"], ["docs.englishReadme", "README.md#english-overview"], ["docs.chineseReadme", "README.zh-CN.md"]] as const) {
    await expect(page.getByRole("link", { name: content.en[key], exact: true })).toHaveAttribute("href", `https://github.com/Z-lab-boop/HarnessScope/blob/main/${path}`);
  }
  await expect(page.locator(".client-matrix")).toContainText("0.162.0-alpha.2");
  await expect(page.locator(".client-matrix")).toContainText("2.1.259");
  await expect(page.getByText("VERIFIED", { exact: true })).toHaveCount(2);
  await expect(page.getByText("PREVIEW", { exact: true })).toHaveCount(2);
  for (const locale of ["en", "zh-CN"] as const) {
    if (locale === "zh-CN") await page.getByRole("button", { name: "切换到中文" }).click();
    const release = page.locator('[data-i18n="docs.releaseBody"]');
    await expect(release).toContainText(locale === "en" ? "Source and tests are public; candidate archives were verified locally." : "源码和测试已经公开；候选归档已在本地验证。");
    await expect(page.locator('[data-i18n="docs.releaseTitle"]')).toHaveText(content[locale]["docs.releaseTitle"]);
  }
});

test("README release claims distinguish public source from local candidate archives", () => {
  for (const file of ["README.md", "README.zh-CN.md"]) {
    const readme = readFileSync(new URL(`../../../${file}`, import.meta.url), "utf8");
    expect(readme).toContain("源码和测试已经公开；四平台候选归档已在本地验证");
    expect(readme).not.toMatch(/候选归档[^。\n]*已[经]?公开/);
    expect(readme).toContain("尚未发布带标签的正式二进制版本");
  }
});

for (const width of [360, 768, 1440]) test(`Docs translation has no overflow at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/docs.html");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.getByRole("button", { name: "切换到中文" }).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

async function expectEnglishNavigation(page: Page) {
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  const navigation = page.getByRole("navigation", { name: "Primary" });
  await expect(navigation).toBeVisible();
  for (const [label, href] of [["Home", "./index.html"], ["Explore", "./explore.html"], ["Docs & Download", "./docs.html"]]) {
    await expect(navigation.getByRole("link", { name: label, exact: true })).toHaveAttribute("href", href);
  }
}

test("JavaScript fallback Home retains headings, captions, navigation and privacy", async ({ page }) => {
  await page.goto("/");
  await expectEnglishNavigation(page);
  await expect(page.getByRole("heading", { name: "Every rule leaves a trail." })).toBeVisible();
  for (const name of ["Discover", "Trace", "Resolve", "Build from source."]) await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
  await expect(page.getByText("Concept photograph", { exact: true }).first()).toBeVisible();
  await expect(page.getByText(content.en["home.localBody"], { exact: true })).toBeVisible();
  await expect(page.getByText(content.en["home.localBoundary"], { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Read the build instructions ↗" }).click();
  await expect(page.getByRole("heading", { name: "Build from source", exact: true })).toBeVisible();
});

test("JavaScript fallback Docs retains commands, navigation and privacy", async ({ page }) => {
  await page.goto("/docs.html");
  await expectEnglishNavigation(page);
  await expect(page.getByRole("heading", { name: "Docs & Download", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Build from source", exact: true })).toBeVisible();
  await expect(page.locator(".terminal-panel code")).toContainText("go build -trimpath -o bin/hscope ./cmd/hscope");
  await expect(page.locator(".terminal-panel code")).toContainText("./bin/hscope serve . --port 0 --open");
  await expect(page.getByText(content.en["docs.localBody"], { exact: true })).toBeVisible();
  await expect(page.getByText(content.en["docs.privacyBody"], { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "GitHub Releases" })).toBeVisible();
  await expect(page.locator('[data-i18n="docs.releaseBody"]')).toContainText("Source and tests are public; candidate archives were verified locally.");
  await expect(page.locator('[data-i18n="docs.releaseTitle"]')).toHaveText("No tagged binary release is published yet.");
  await expect(page.getByRole("link", { name: "English overview", exact: true })).toHaveAttribute("href", "https://github.com/Z-lab-boop/HarnessScope/blob/main/README.md#english-overview");
});

test.describe("Explore JavaScript fallback", () => {
  test.use({ javaScriptEnabled: false });
  test("explains that interaction requires JavaScript", async ({ page }) => {
    await page.goto("/explore.html");
    await expectEnglishNavigation(page);
    await expect(page.getByText("JavaScript is required for the interactive walkthrough. Navigation and documentation remain available.", { exact: true })).toBeVisible();
    await expect(page.locator("[data-explore-workbench]")).toBeHidden();
  });
});
