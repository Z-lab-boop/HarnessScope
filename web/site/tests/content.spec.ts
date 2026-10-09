import { test, expect } from "@playwright/test";

test("every declared translation renders in both languages", async ({ page }) => {
  for (const path of ["/", "/explore.html", "/docs.html"]) {
    await page.goto(path);
    expect(await page.locator("[data-i18n]").count()).toBeGreaterThan(0);
    expect(await page.locator("[data-i18n]").evaluateAll(elements => elements.some(element => /undefined|missing/i.test(element.textContent ?? "")))).toBe(false);
    await page.getByRole("button", { name: "切换到中文" }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    expect((await page.locator("main").innerText()).trim().length).toBeGreaterThan(200);
    await page.getByRole("button", { name: "Switch to English" }).click();
  }
});

test("Docs states the truthful release boundary", async ({ page }) => {
  await page.goto("/docs.html");
  await expect(page.getByRole("heading", { name: "Build from source" })).toBeVisible();
  await expect(page.getByText("Download v0.2", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "GitHub Releases" })).toHaveAttribute("href", /releases/);
  await expect(page.getByText("No tagged binary release is published yet.", { exact: true })).toBeVisible();
});

test.describe("JavaScript fallback", () => {
  test.use({ javaScriptEnabled: false });
  test("keeps Home and Docs authoritative", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Every rule leaves a trail." })).toBeVisible();
    await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
    await page.goto("/docs.html");
    await expect(page.getByRole("heading", { name: "Build from source" })).toBeVisible();
    await expect(page.locator(".terminal-panel code")).toContainText("go build -trimpath -o bin/hscope ./cmd/hscope");
    await expect(page.getByText("Local and offline", { exact: true })).toBeVisible();
    await page.goto("/explore.html");
    await expect(page.getByText("JavaScript is required for the interactive walkthrough. Navigation and documentation remain available.", { exact: true })).toBeVisible();
  });
});
