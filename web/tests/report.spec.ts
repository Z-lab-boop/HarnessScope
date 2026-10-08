import { execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";
import { resolve } from "node:path";
import { test, expect } from "@playwright/test";

const repository = resolve(import.meta.dirname, "../..");
const binary = resolve(repository, "bin/hscope");
const input = resolve(import.meta.dirname, "fixture-report.json");
const output = resolve(import.meta.dirname, "../test-results/report.html");

test.beforeAll(() => {
  execFileSync(binary, ["report", "--from", input, "--html", output], { stdio: "pipe" });
});

test("report stays offline and keyboard accessible", async ({ page }) => {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (!request.url().startsWith("file:")) requests.push(request.url());
  });
  await page.goto(pathToFileURL(output).href);
  await expect(page.getByRole("heading", { name: "Effective configuration provenance" })).toBeVisible();
  await page.getByLabel("Severity").focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.locator("tr[data-severity=HIGH]")).toBeVisible();
  await page.getByRole("button", { name: "Toggle embedded JSON" }).click();
  await expect(page.locator("#raw-data")).toContainText('"schema_version": "1.0.0"');
  await page.getByRole("button", { name: "Toggle embedded JSON" }).click();
  expect(requests).toEqual([]);
  if (process.platform === "linux") {
    await expect(page).toHaveScreenshot("report.png", { fullPage: true, maxDiffPixelRatio: 0.03 });
  }
});

test("report remains readable without JavaScript", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto(pathToFileURL(output).href);
  await expect(page.getByRole("table")).toContainText("PATH-0001");
  await expect(page.getByText("CONFIRMED")).toBeVisible();
  await context.close();
});
