import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { parseDemo } from "../src/demo-schema.js";

const fixture = () => JSON.parse(readFileSync(new URL("../data/demo.json", import.meta.url), "utf8"));

test("Explore is explicitly synthetic and supports keyboard inspection", async ({ page }) => {
  await page.goto("/explore.html");
  await expect(page.getByText("Synthetic interactive walkthrough", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Inspect project instructions", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("complementary", { name: "Evidence inspector" })).toContainText("./AGENTS.md");
});

test("Explore filters evidence and previews without mutating", async ({ page }) => {
  const methods: string[] = [];
  page.on("request", request => methods.push(request.method()));
  await page.goto("/explore.html");
  await page.getByRole("button", { name: "Show conflicts" }).click();
  await expect(page.getByRole("region", { name: "Conflict evidence" })).toContainText("Fictional duplicate MCP declaration");
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  await expect(page.getByText("Preview only · nothing was changed", { exact: true })).toBeVisible();
  expect(methods.every(method => method === "GET")).toBe(true);
});

test("demo schema rejects malformed relationships and private data", () => {
  const duplicate = fixture();
  duplicate.nodes[1].id = duplicate.nodes[0].id;
  expect(() => parseDemo(duplicate)).toThrow("Invalid synthetic demo");

  const absolute = fixture();
  absolute.nodes[0].origin = "/Users/private/.codex/config.toml";
  expect(() => parseDemo(absolute)).toThrow("Invalid synthetic demo");

  const unknownEvidence = fixture();
  unknownEvidence.nodes[0].evidence = "PROVEN";
  expect(() => parseDemo(unknownEvidence)).toThrow("Invalid synthetic demo");

  const credential = fixture();
  credential.fix_preview.patch[0] = "api_key=sk-example-not-a-secret";
  expect(() => parseDemo(credential)).toThrow("Invalid synthetic demo");

  const missingEndpoint = fixture();
  missingEndpoint.edges[0].to = "missing-node";
  expect(() => parseDemo(missingEndpoint)).toThrow("Invalid synthetic demo");
});

test("Explore exposes no private path, address or credential canary", async ({ page }) => {
  await page.goto("/explore.html");
  const text = await page.locator("body").innerText();
  for (const canary of ["/Users/", "/home/", "sk-", "@example.com", "session_token"]) expect(text).not.toContain(canary);
});
