import { defineConfig, devices } from "@playwright/test";
import { fileURLToPath } from "node:url";

export default defineConfig({
  testDir: "./tests",
  outputDir: "../test-results/site",
  workers: 1,
  use: { ...devices["Desktop Chrome"], baseURL: "http://127.0.0.1:4177", channel: process.env.PLAYWRIGHT_CHANNEL },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
  webServer: {
    command: "node site/test-server.mjs ../../dist/site",
    cwd: fileURLToPath(new URL("../", import.meta.url)),
    url: "http://127.0.0.1:4177",
    reuseExistingServer: false,
  },
});
