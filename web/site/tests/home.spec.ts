import { test, expect } from "@playwright/test";

test("home tells the product story with truthful image roles", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Every rule leaves a trail." })).toBeVisible();
  await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
  await expect(page.getByText("Concept photograph", { exact: true }).first()).toBeVisible();
  for (const name of ["Discover", "Trace", "Resolve"]) await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByText("VERIFIED", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("PREVIEW", { exact: true }).first()).toBeVisible();
});

for (const width of [360, 768, 1440]) test(`home has no overflow at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test("home images load at the project subpath with truthful alternatives", async ({ page }) => {
  const requests: string[] = [];
  page.on("request", request => requests.push(request.url()));
  await page.goto("/HarnessScope/");
  for (const image of await page.locator("main img").all()) {
    await image.scrollIntoViewIfNeeded();
    await expect.poll(() => image.evaluate((item: HTMLImageElement) => item.complete && item.naturalWidth > 0 && item.naturalHeight > 0)).toBe(true);
    expect(await image.getAttribute("src")).toMatch(/^\.\/assets\//);
  }
  await expect(page.locator('img[src*="generated/"]')).toHaveCount(3);
  for (const image of await page.locator('img[src*="generated/"]').all()) await expect(image).toHaveAttribute("alt", "");
  await expect(page.locator(".product-capture")).toHaveAttribute("alt", "HarnessScope synthetic dashboard overview");
  expect(requests.every(url => new URL(url).origin === new URL(page.url()).origin && new URL(url).pathname.startsWith("/HarnessScope/"))).toBe(true);
});

test("home orbit supports keyboard navigation without hiding the story", async ({ page }) => {
  await page.goto("/");
  const discover = page.getByRole("button", { name: "Discover", exact: true });
  await discover.focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "Trace", exact: true })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#trace")).toBeFocused();
  await expect(page).toHaveURL(/#trace$/);
  await expect(page.getByRole("button", { name: "Trace", exact: true })).toHaveAttribute("aria-current", "step");
  await page.getByRole("button", { name: "Resolve", exact: true }).focus();
  await page.keyboard.press("Space");
  await expect(page.locator("#resolve")).toBeFocused();
  for (const id of ["discover", "trace", "resolve"]) await expect(page.locator(`#${id}`)).toBeVisible();
});

for (const width of [360, 1440]) test(`home keeps clicked orbit chapters active after observer delivery at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  for (const name of ["Trace", "Resolve"]) {
    await page.getByRole("button", { name, exact: true }).click();
    // Immediate click assertions can pass before IntersectionObserver updates.
    await page.waitForTimeout(250);
    const id = name.toLowerCase();
    await expect(page).toHaveURL(new RegExp(`#${id}$`));
    await expect(page.locator(`#${id}`)).toBeFocused();
    await expect(page.getByRole("button", { name, exact: true })).toHaveAttribute("aria-current", "step");
    await expect(page.locator(`#${id}`)).toHaveClass(/is-current/);
    await expect(page.locator("[data-orbit-target][aria-current]")).toHaveCount(1);
    await expect(page.locator("[data-story-section].is-current")).toHaveCount(1);
  }
});

test("home removes nonessential animation under reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  const animations = await page.locator("body *").evaluateAll(elements => elements.flatMap(element => [null, "::before", "::after"].map(pseudo => getComputedStyle(element, pseudo).animationName)));
  expect(animations.every(name => name === "none")).toBe(true);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
});

test("home translates body, tiers, captions and accessible names explicitly", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("/");
  const english = await page.locator('main [data-i18n], main [data-i18n-aria], main [data-i18n-alt]').evaluateAll(elements => elements.map(element => element.textContent + element.getAttribute("aria-label") + element.getAttribute("alt")));
  await page.getByRole("button", { name: "切换到中文" }).click();
  const chinese = await page.locator('main [data-i18n], main [data-i18n-aria], main [data-i18n-alt]').evaluateAll(elements => elements.map(element => element.textContent + element.getAttribute("aria-label") + element.getAttribute("alt")));
  expect(chinese.every((value, index) => value !== english[index])).toBe(true);
  await expect(page.getByRole("heading", { name: "每条规则，都有迹可循。" })).toBeVisible();
  await expect(page.getByText("合成数据仪表盘截图", { exact: true })).toBeVisible();
  await expect(page.getByRole("group", { name: "探索配置证据章节" })).toBeVisible();
  await expect(page.locator(".product-capture")).toHaveAttribute("alt", "HarnessScope 合成数据仪表盘总览");
  await page.getByRole("button", { name: "Switch to English" }).click();
  await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("home retains English evidence and chapter links without JavaScript", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 360, height: 900 } });
  const page = await context.newPage();
  await page.goto("/");
  await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Trace", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Trace", exact: true }).click();
  await expect(page).toHaveURL(/#trace$/);
  await expect(page.getByRole("heading", { name: "Trace", exact: true })).toBeVisible();
  await context.close();
});
