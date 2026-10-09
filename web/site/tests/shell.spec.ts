import { test, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";

test.use({ locale: "zh-CN" });

for (const path of ["/", "/explore.html", "/docs.html"]) {
  test(`${path} has an English-first accessible shell`, async ({ page }) => {
    const requests: string[] = [];
    page.on("request", request => requests.push(request.url()));
    await page.goto(path);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
    await expect(page.getByRole("button", { name: "切换到中文" })).toBeVisible();
    expect(requests.every(url => new URL(url).origin === new URL(page.url()).origin)).toBe(true);
  });
}

test("language selection is explicit and persists without cookies", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "切换到中文" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("navigation", { name: "主导航" })).toBeVisible();
  expect(await page.context().cookies()).toEqual([]);
  await page.goto("/docs.html");
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("link", { name: "文档与下载", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Switch to English" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
});

test("English-first ignores browser locale and invalid stored values", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("harnessscope.locale", "fr"));
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  expect(await page.evaluate(() => localStorage.getItem("harnessscope.locale"))).toBe("fr");
});

test("repository subpath serves CSS, JavaScript, data and images", async ({ page }) => {
  // Temporary build-output fixtures exercise future asset mounts without
  // requiring Task 2's production images or Task 4's dataset.
  const out = new URL("../../../dist/site/", import.meta.url);
  await mkdir(new URL("data/", out), { recursive: true });
  await writeFile(new URL("data/shell-test.json", out), '{"synthetic":true}');
  await writeFile(new URL("assets/shell-test.svg", out), '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>');
  const responses: { url: string; status: number }[] = [];
  page.on("response", response => responses.push({ url: response.url(), status: response.status() }));
  await page.goto("/HarnessScope/explore.html");
  await expect(page.getByRole("button", { name: "切换到中文" })).toBeVisible();
  const assets = await page.evaluate(async () => {
    const urls = ["./assets/site.css", "./assets/site.js", "./data/shell-test.json", "./assets/shell-test.svg"];
    return Promise.all(urls.map(async url => {
      const response = await fetch(url);
      return { url: response.url, status: response.status, type: response.headers.get("content-type") };
    }));
  });
  expect(assets.map(asset => asset.status)).toEqual([200, 200, 200, 200]);
  expect(assets.map(asset => asset.type)).toEqual(["text/css; charset=utf-8", "text/javascript; charset=utf-8", "application/json; charset=utf-8", "image/svg+xml"]);
  expect(responses.every(response => response.status === 200 && new URL(response.url).pathname.startsWith("/HarnessScope/"))).toBe(true);
});

test("shell remains readable without JavaScript and fits a narrow viewport", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 360, height: 800 } });
  const page = await context.newPage();
  for (const path of ["/", "/explore.html", "/docs.html"]) {
    await page.goto(path);
    await expect(page.getByRole("main")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await context.close();
});

test("static server rejects encoded traversal", async ({ request }) => {
  const response = await request.get("/HarnessScope/%2e%2e%2fpackage.json");
  expect(response.status()).toBe(400);
});
