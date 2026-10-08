import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  outputDir: "./test-results",
  snapshotPathTemplate: "{testDir}/__snapshots__/{arg}{ext}",
  use: {
    ...devices["Desktop Chrome"],
    viewport: { width: 1440, height: 1000 },
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
