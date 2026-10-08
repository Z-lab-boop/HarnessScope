import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  outputDir: "../test-results/dashboard",
  workers: 1,
  snapshotPathTemplate: "{testDir}/__snapshots__/{arg}{ext}",
  use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 }, channel: process.env.PLAYWRIGHT_CHANNEL },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
