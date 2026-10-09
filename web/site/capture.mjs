import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const repository = fileURLToPath(new URL("../../", import.meta.url));
const web = fileURLToPath(new URL("../", import.meta.url));
const base = "http://127.0.0.1:4177";
const server = spawn(process.execPath, ["site/test-server.mjs", "../../dist/site"], { cwd: web, stdio: ["ignore", "pipe", "pipe"] });
let diagnostics = "";
server.stderr.on("data", chunk => { diagnostics += chunk.toString(); });

async function ready() {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (server.exitCode !== null) throw new Error(`capture server exited early: ${diagnostics}`);
    try { const response = await fetch(base); if (response.ok) return; } catch { /* retry while the loopback server starts */ }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(`capture server did not become ready: ${diagnostics}`);
}

const captures = [
  { path: "/", target: "hero", file: "site-hero.png" },
  { path: "/explore.html", target: "explore", file: "site-explore.png" },
  { path: "/docs.html", target: "docs", file: "site-docs.png" },
];

let browser;
try {
  await ready();
  browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 }, reducedMotion: "reduce", colorScheme: "light" });
  await page.addInitScript(() => localStorage.removeItem("harnessscope.locale"));
  for (const capture of captures) {
    await page.goto(`${base}${capture.path}`, { waitUntil: "networkidle" });
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(Array.from(document.images).map(image => image.complete ? Promise.resolve() : image.decode()));
    });
    const locator = page.locator(`[data-capture="${capture.target}"]`);
    await locator.waitFor({ state: "visible" });
    await locator.screenshot({ path: join(repository, "docs/assets", capture.file), animations: "disabled", scale: "css" });
  }
} finally {
  await browser?.close();
  server.kill("SIGTERM");
}
