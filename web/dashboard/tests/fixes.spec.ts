import { test, expect, type Page } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile } from "node:fs/promises";

const token = "fix-session-token";
const fixture = JSON.parse(await readFile(new URL("./fixture-state.json", import.meta.url), "utf8"));
const plans = ["SAFE", "REVIEW", "MANUAL"].map((risk) => ({ id: `FIX-${risk}`, risk, finding_rule_id: "duplicate", edits: [{ source_id: "a", target_path: "./AGENTS.md", operation: "remove_byte_range", redacted_patch: '<img src="https://example.invalid/leak"> [REDACTED]', replacement: "RAW_REPLACEMENT_CANARY" }], backup_required: true, preconditions: ["Inspect the current plan"], postconditions: ["Duplicate removed"], rollback: ["Restore original bytes and mode"] }));
plans.push({ ...plans[0], id: "FIX-SAFE-TWO" });
let server: Server, base: string, revision: number, applied: boolean, mode: string;
let calls: { path: string; body: any }[];
let release: (() => void) | undefined;
test.beforeAll(async () => {
  server = createServer(async (req, res) => {
    const path = new URL(req.url!, "http://localhost").pathname;
    res.setHeader("Cache-Control", "no-store");
    if (path.startsWith("/api/")) {
      let raw = ""; for await (const chunk of req) raw += chunk;
      const body = raw ? JSON.parse(raw) : null; calls.push({ path, body });
      res.setHeader("Content-Type", "application/json");
      if (req.headers["x-harnessscope-token"] !== token) { res.writeHead(401).end(JSON.stringify({ code: "unauthorized", message: "Session required", details: {} })); return; }
      if (body && body.revision !== revision) { res.writeHead(409).end(JSON.stringify({ code: "stale_revision", message: "Refresh the snapshot", details: {} })); return; }
      if (path === "/api/v1/fixes/plan") { res.end(JSON.stringify(applied ? [] : plans)); return; }
      if (path === "/api/v1/state" && mode === "refresh-fails") { res.writeHead(503).end(JSON.stringify({ code: "unavailable", message: "Snapshot unavailable", details: {} })); return; }
      if (path === "/api/v1/fixes/apply") {
        if (mode === "hold") await new Promise<void>((resolve) => { release = resolve; });
        if (mode === "conflict") { res.writeHead(409).end(JSON.stringify({ code: "target_changed", message: "Target changed", details: {} })); return; }
        applied = true; revision++;
      }
      if (path === "/api/v1/rollback") { applied = false; revision++; }
      if (path === "/api/v1/rescan") revision++;
      res.end(JSON.stringify({ ...fixture, revision, fix_plans: applied ? [] : plans, backups: applied ? [{ id: "backup-001", created_at: fixture.scanned_at, file_count: 1 }] : [] }));
      return;
    }
    const name = path === "/" ? "index.html" : path.slice("/assets/".length);
    if (!["index.html", "dashboard.js", "dashboard.css"].includes(name)) { res.writeHead(404).end(); return; }
    res.setHeader("Content-Type", name.endsWith(".js") ? "text/javascript" : name.endsWith(".css") ? "text/css" : "text/html");
    res.end(await readFile(new URL(`../../../internal/server/assets/${name}`, import.meta.url)));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address(); if (!address || typeof address === "string") throw new Error("No port");
  base = `http://127.0.0.1:${address.port}`;
});
test.beforeEach(() => { revision = 7; applied = false; mode = "ok"; calls = []; release = undefined; });
test.afterAll(async () => { release?.(); await new Promise<void>((resolve) => server.close(() => resolve())); });
const open = async (page: Page) => { await page.goto(`${base}/?view=fixes#token=${token}`); await expect(page.getByRole("heading", { name: "Dry fix plans" })).toBeVisible(); };
const confirmApply = async (page: Page) => { await page.getByRole("checkbox", { name: "Select FIX-SAFE", exact: true }).check(); await page.getByRole("button", { name: "Apply selected SAFE fixes" }).click(); };

test("dry plans render text only and restrict selection to SAFE", async ({ page }) => {
  const requests: string[] = []; page.on("request", (r) => requests.push(r.url()));
  await open(page);
  await expect(page.getByRole("button", { name: "Apply selected SAFE fixes" })).toBeDisabled();
  for (const risk of ["SAFE", "REVIEW", "MANUAL"]) await expect(page.getByRole("article", { name: `FIX-${risk}`, exact: true })).toContainText(risk);
  await expect(page.getByRole("checkbox", { name: "Select FIX-REVIEW" })).toBeDisabled();
  await expect(page.getByRole("checkbox", { name: "Select FIX-MANUAL" })).toBeDisabled();
  await expect(page.getByRole("article", { name: "FIX-REVIEW" })).toContainText("Browser apply unavailable");
  await expect(page.getByRole("article", { name: "FIX-MANUAL" })).toContainText("Browser apply unavailable");
  await expect(page.locator("pre").first()).toHaveText(plans[0].edits[0].redacted_patch);
  await expect(page.locator("img")).toHaveCount(0);
  await expect(page.locator("body")).not.toContainText("RAW_REPLACEMENT_CANARY");
  await page.getByRole("button", { name: "Refresh dry plan" }).click();
  await expect.poll(() => calls.filter((c) => c.path.endsWith("/rescan")).length).toBe(1);
  await expect(page.getByText("REV 8", { exact: true })).toBeVisible();
  expect(requests.every((url) => new URL(url).origin === base)).toBe(true);
});

test("apply confirmation traps focus, cancels with Escape, and updates revision and history once", async ({ page }) => {
  await open(page); await confirmApply(page);
  const dialog = page.getByRole("dialog", { name: "Apply SAFE fixes?" });
  await expect(dialog).toContainText("1 file"); await expect(dialog).toContainText("backup");
  await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused();
  await page.keyboard.press("Shift+Tab"); await expect(dialog.getByRole("button", { name: "Confirm apply" })).toBeFocused();
  await page.keyboard.press("Tab"); await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused();
  await page.keyboard.press("Escape"); await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Apply selected SAFE fixes" })).toBeFocused();
  expect(calls.filter((c) => c.path.endsWith("/apply"))).toHaveLength(0);
  await page.getByRole("button", { name: "Apply selected SAFE fixes" }).click();
  mode = "hold"; await page.getByRole("button", { name: "Confirm apply" }).click();
  await expect(page.getByRole("button", { name: "Confirm apply" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Rescan workspace" })).toBeDisabled();
  await page.keyboard.press("Escape"); await expect(dialog).toBeVisible();
  await expect.poll(() => Boolean(release)).toBe(true); release!();
  await expect(page.getByText("REV 8", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Rollback backup-001" })).toBeVisible();
  await expect(page.getByRole("status")).toContainText("Applied");
  await expect(page.locator(".fix-feedback")).toBeVisible();
  await expect(page.locator(".fix-feedback")).toContainText("Applied selected SAFE fixes. Scan revision 8 loaded.");
  expect(calls.filter((c) => c.path.endsWith("/apply"))).toEqual([{ path: "/api/v1/fixes/apply", body: { revision: 7, fix_ids: ["FIX-SAFE"] } }]);
  await page.getByRole("button", { name: "Rollback backup-001" }).click();
  const rollback = page.getByRole("dialog", { name: "Rollback backup-001?" });
  await expect(rollback).toContainText("overwrite");
  await expect(rollback.getByRole("button", { name: "Confirm rollback" })).toBeDisabled();
  await rollback.getByLabel("Type backup ID to confirm").fill("backup-wrong");
  await expect(rollback.getByRole("button", { name: "Confirm rollback" })).toBeDisabled();
  await rollback.getByLabel("Type backup ID to confirm").fill("backup-001");
  await rollback.getByRole("button", { name: "Confirm rollback" }).click();
  await expect(page.getByText("REV 9", { exact: true })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Select FIX-SAFE", exact: true })).not.toBeChecked();
  expect(calls[calls.length - 1]).toEqual({ path: "/api/v1/rollback", body: { revision: 8, backup_id: "backup-001" } });
});

test("stale apply refreshes without replay and requires reviewing a fresh selection", async ({ page }) => {
  await open(page); await confirmApply(page); revision = 11;
  await page.getByRole("button", { name: "Confirm apply" }).click();
  await expect(page.getByText("REV 11", { exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Rescan");
  await expect(page.getByRole("button", { name: "Apply selected SAFE fixes" })).toBeDisabled();
  expect(calls.filter((c) => c.path.endsWith("/apply"))).toHaveLength(1);
  expect(applied).toBe(false);
});

test("target conflict and failed stale refresh stay recoverable", async ({ page }) => {
  await open(page); await confirmApply(page); mode = "conflict";
  await page.getByRole("button", { name: "Confirm apply" }).click();
  await expect(page.getByRole("alert")).toContainText("Rescan");
  await expect(page.getByRole("button", { name: "Rescan workspace" })).toBeEnabled();
  await confirmApply(page); revision = 12; mode = "refresh-fails";
  await page.getByRole("button", { name: "Confirm apply" }).click();
  await expect(page.getByRole("alert")).toContainText("refresh failed");
  await expect(page.getByRole("button", { name: "Rescan workspace" })).toBeEnabled();
  expect(applied).toBe(false);
});

test("shared target count and 768px confirmation stay readable", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 768, height: 900 });
  await open(page);
  await page.screenshot({ path: testInfo.outputPath("fix-center-768.png"), fullPage: true });
  await page.getByRole("checkbox", { name: "Select FIX-SAFE", exact: true }).check();
  await page.getByRole("checkbox", { name: "Select FIX-SAFE-TWO", exact: true }).check();
  await page.getByRole("button", { name: "Apply selected SAFE fixes" }).click();
  const dialog = page.getByRole("dialog", { name: "Apply SAFE fixes?" });
  await expect(dialog).toContainText("2 SAFE fixes will change 1 file");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("fix-confirmation-768.png") });
  await page.keyboard.press("Escape");
  expect(calls.filter((c) => c.path.endsWith("/apply"))).toHaveLength(0);
});
