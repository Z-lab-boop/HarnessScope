import { chromium } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { startCaptureServer } from "./capture-server.mjs";

const repository = fileURLToPath(new URL("../../", import.meta.url));

const captures = [
  { path: "/", target: "hero", file: "site-hero.png" },
  { path: "/explore.html", target: "explore", file: "site-explore.png" },
  { path: "/docs.html", target: "docs", file: "site-docs.png" },
];

let browser;
let server;
try {
  const connection = await startCaptureServer();
  server = connection.server;
  const base = connection.base;
  browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 }, deviceScaleFactor: 1, locale: "en-US", timezoneId: "UTC", reducedMotion: "reduce", colorScheme: "light" });
  await page.addInitScript(() => localStorage.setItem("harnessscope.locale", "en"));
  for (const capture of captures) {
    await page.goto(`${base}${capture.path}`, { waitUntil: "networkidle" });
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(Array.from(document.images).map(async image => {
        await image.decode();
        if (!image.complete || image.naturalWidth === 0 || image.naturalHeight === 0) throw new Error(`Image did not load: ${image.getAttribute("src")}`);
      }));
    });
    if (await page.locator("html").getAttribute("lang") !== "en") throw new Error(`${capture.file}: capture must be English`);
    const locator = page.locator(`[data-capture="${capture.target}"]`);
    if (await locator.count() !== 1) throw new Error(`${capture.file}: expected exactly one capture surface`);
    await locator.waitFor({ state: "visible" });
    await locator.screenshot({ path: join(repository, "docs/assets", capture.file), animations: "disabled", scale: "css" });
  }
} finally {
  await browser?.close();
  server?.kill("SIGTERM");
}
