import { test, expect } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile, readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { APIClient } from "../src/api.js";

const token = "synthetic-session-token";
const fixture = JSON.parse(await readFile(new URL("./fixture-state.json", import.meta.url), "utf8"));
const assets = new URL("../../../internal/server/assets/", import.meta.url);
let server: Server;
let base: string;
let mode = "ok";
let serverRevision = fixture.revision;
let calls: { path: string; token: string | undefined; body: string }[] = [];
let requests: { method: string; url: string }[] = [];

test.beforeAll(async () => {
  server = createServer(async (req, res) => {
    res.setHeader("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'");
    res.setHeader("Cache-Control", "no-store");
    const path = new URL(req.url!, "http://localhost").pathname;
    if (path.startsWith("/api/")) {
      let body = "";
      for await (const chunk of req) body += chunk;
      calls.push({ path, token: req.headers["x-harnessscope-token"] as string | undefined, body });
      requests.push({ method: req.method!, url: req.url! });
      if (mode === "redirect") { res.writeHead(302, { Location: "https://example.invalid/leak" }).end(); return; }
      if (mode === "html") {
        res.writeHead(502, { "Content-Type": "text/html" });
        res.end("<h1>private upstream error</h1>");
        return;
      }
      res.setHeader("Content-Type", "application/json");
      if (mode === "malformed") { res.statusCode = 500; res.end('{"message":"bad"}'); return; }
      if (mode === "invalid-json") { res.end('{'); return; }
      if (req.headers["x-harnessscope-token"] !== token || mode === "unauthorized") {
        res.statusCode = 401;
        res.end(JSON.stringify({ code: "unauthorized", message: "Session is no longer valid.", details: {} }));
      } else {
        if (path === "/api/v1/rescan") {
          if (JSON.parse(body).revision !== serverRevision) {
            res.statusCode = 409;
            res.end(JSON.stringify({ code: "stale_revision", message: "The dashboard revision changed. Refresh and retry.", details: {} }));
            return;
          }
          serverRevision++;
        }
        if (path === "/api/v1/state" && mode === "refresh-fails") {
          res.statusCode = 503;
          res.end(JSON.stringify({ code: "unavailable", message: "The local snapshot is temporarily unavailable.", details: {} }));
          return;
        }
        if (path.endsWith("export")) { res.setHeader("Content-Type", "application/zip"); res.end(Buffer.from([80, 75, 3, 4])); return; }
        if (["/api/v1/backups", "/api/v1/fixes/plan"].includes(path) || (path === "/api/v1/snapshots" && req.method === "GET")) { res.end("[]"); return; }
        if (path === "/api/v1/snapshots") { res.end(JSON.stringify({ name: "base", schema_version: "1.0.0", created_at: fixture.scanned_at })); return; }
        if (path === "/api/v1/explain") { res.end(JSON.stringify({ node: { id: "node-a", type: "CLIENT", client: "codex", display_name: "codex", adapter_confidence: "CONFIRMED" }, edges: [] })); return; }
        if (path === "/api/v1/compare") { res.end(JSON.stringify({ left: "codex", right: "opencode", rows: [] })); return; }
        const data = structuredClone(fixture);
        data.revision = serverRevision;
        if (mode === "empty") {
          data.result.analysis = { clients: null, sources: null, graph: { nodes: null, edges: null }, findings: null };
        }
        if (mode === "hostile") data.result.analysis.clients[0].id = '<img src="https://example.invalid/leak">';
        res.end(JSON.stringify(data));
      }
      return;
    }
    const name = path === "/" ? "index.html" : path.slice("/assets/".length);
    if (!["index.html", "dashboard.js", "dashboard.css"].includes(name)) { res.writeHead(404).end(); return; }
    res.setHeader("Content-Type", name.endsWith(".js") ? "text/javascript" : name.endsWith(".css") ? "text/css" : "text/html");
    res.end(await readFile(new URL(name, assets)));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Fixture did not bind");
  base = `http://127.0.0.1:${address.port}`;
});
test.afterAll(async () => { await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())); });
test.beforeEach(() => { mode = "ok"; serverRevision = fixture.revision; calls = []; requests = []; });

test("consumes the fragment once, authenticates same-origin requests, and keeps the token out of storage", async ({ page }) => {
  const requests: string[] = [];
  page.on("request", (request) => requests.push(request.url()));
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("heading", { name: "Configuration observatory" })).toBeVisible();
  expect(new URL(page.url()).hash).toBe("");
  expect(calls).toEqual([{ path: "/api/v1/state", token, body: "" }]);
  await page.getByRole("button", { name: "Rescan workspace" }).click();
  await expect(page.getByText("REV 8", { exact: true })).toBeVisible();
  expect(calls[1]).toEqual({ path: "/api/v1/rescan", token, body: '{"revision":7}' });
  expect(requests.every((url) => new URL(url).origin === base && !url.includes(token))).toBe(true);
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length, cookie: document.cookie }))).toEqual({ local: 0, session: 0, cookie: "" });
  expect(await page.locator("body").textContent()).not.toContain(token);
  await page.reload();
  await expect(page.getByRole("alert")).toContainText("launch URL");
  expect(calls).toHaveLength(2);
});

test("renders snapshot risk cards, client evidence tiers and navigable shell landmarks", async ({ page }) => {
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
  await expect(page.getByRole("complementary", { name: "Inspector" })).toBeVisible();
  for (const title of ["Client coverage", "Findings", "Estimated context", "Portability"]) {
    await expect(page.getByRole("region", { name: title, exact: true })).toBeVisible();
  }
  await expect(page.getByText("VERIFIED", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("PREVIEW", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("HIGH · 1", { exact: true })).toBeVisible();
  await expect(page.getByText("450–900", { exact: true })).toBeVisible();
  await expect(page.getByText("Snapshot only · no live monitoring", { exact: true })).toBeVisible();
  for (const name of ["Graph", "Findings", "Compare", "Fix Center", "Drift", "Export", "Overview"]) {
    await page.getByRole("link", { name, exact: true }).click();
    await expect(page.getByRole("link", { name, exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
  }
  await page.goBack();
  await expect(page.getByRole("link", { name: "Export", exact: true })).toHaveAttribute("aria-current", "page");
});

test("stale revision refreshes the authenticated snapshot and waits for an explicit retry", async ({ page }) => {
  const urls: string[] = [];
  page.on("request", (request) => urls.push(request.url()));
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByText("REV 7", { exact: true })).toBeVisible();
  // An independent client advances the server after this page loaded its snapshot.
  serverRevision = 11;
  await page.getByRole("button", { name: "Rescan workspace" }).click();
  await expect(page.getByText("REV 11", { exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Review the refreshed snapshot and retry");
  await expect(page.getByRole("status")).toContainText("Scan revision 11 loaded");
  expect(calls).toEqual([
    { path: "/api/v1/state", token, body: "" },
    { path: "/api/v1/rescan", token, body: '{"revision":7}' },
    { path: "/api/v1/state", token, body: "" },
  ]);
  expect(serverRevision).toBe(11);
  expect(new URL(page.url()).hash).toBe("");
  expect(urls.every((url) => new URL(url).origin === base && !url.includes(token))).toBe(true);
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length, cookie: document.cookie }))).toEqual({ local: 0, session: 0, cookie: "" });
  await page.getByRole("button", { name: "Rescan workspace" }).click();
  await expect(page.getByText("REV 12", { exact: true })).toBeVisible();
  expect(calls[3]).toEqual({ path: "/api/v1/rescan", token, body: '{"revision":11}' });
  await expect(page.getByRole("alert")).toBeHidden();
});

test("stale revision refresh failure stays recoverable without replaying the mutation", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByText("REV 7", { exact: true })).toBeVisible();
  serverRevision = 11;
  mode = "refresh-fails";
  await page.getByRole("button", { name: "Rescan workspace" }).click();
  await expect(page.getByRole("alert")).toContainText("temporarily unavailable");
  await expect(page.getByRole("button", { name: "Rescan workspace" })).toBeEnabled();
  expect(calls.map((call) => call.path)).toEqual(["/api/v1/state", "/api/v1/rescan", "/api/v1/state"]);
  expect(serverRevision).toBe(11);
  mode = "ok";
  await page.getByRole("button", { name: "Rescan workspace" }).click();
  await expect(page.getByText("REV 11", { exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("keyboard reaches the workbench and selection inspector with visible focus", async ({ page }) => {
  await page.goto(`${base}/#token=${token}`);
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to workbench" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
  const client = page.getByRole("button", { name: "Inspect codex" });
  await page.keyboard.press("Tab");
  await expect(client).toBeFocused();
  expect(await client.evaluate((el) => getComputedStyle(el).outlineStyle)).not.toBe("none");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("complementary", { name: "Inspector" })).toContainText("codex");
  await expect(client).toBeFocused();
});

test("768px layout preserves content without horizontal page overflow and honors reduced motion", async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("button", { name: "Inspect opencode" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(await page.locator(".signal-dot").first().evaluate((el) => getComputedStyle(el).animationName)).toBe("none");
  await page.getByRole("link", { name: "Export", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
});

test("non-JSON failures stay readable without rendering the upstream payload", async ({ page }) => {
  mode = "html";
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("alert")).toContainText("JSON");
  await expect(page.locator("body")).not.toContainText("private upstream error");
  await expect(page.getByRole("button", { name: "Rescan workspace" })).toBeDisabled();
});

test("empty server slices and hostile strings cannot break or inject the shell", async ({ page }) => {
  mode = "empty";
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByText("No clients in this scan.", { exact: true })).toBeVisible();
  mode = "hostile";
  await page.goto(`${base}/?case=hostile#token=${token}`);
  await expect(page.getByRole("button", { name: 'Inspect <img src="https://example.invalid/leak">' })).toBeVisible();
  await expect(page.locator("img")).toHaveCount(0);
});

test("every typed API method preserves route, query encoding and revision payload", async () => {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: { location: { origin: base }, fetch: globalThis.fetch } });
  try {
    const api = new APIClient(token);
    expect((await api.explain("node/a?b")).node.id).toBe("node-a");
    expect((await api.compare("codex", "opencode")).rows).toEqual([]);
    expect(await api.backups()).toEqual([]);
    expect(await api.snapshots()).toEqual([]);
    expect(await api.planFixes(7)).toEqual([]);
    expect((await api.applyFixes(7, ["safe-a"])).revision).toBe(7);
    expect((await api.rollback(7, "backup-a")).revision).toBe(7);
    expect((await api.saveSnapshot(7, "base")).name).toBe("base");
    expect((await api.drift(7, "base")).revision).toBe(7);
    expect(Array.from(new Uint8Array(await (await api.export(7, "base")).arrayBuffer()))).toEqual([80, 75, 3, 4]);
    expect(requests).toEqual([
      { method: "GET", url: "/api/v1/explain?id=node%2Fa%3Fb" },
      { method: "GET", url: "/api/v1/compare?left=codex&right=opencode" },
      { method: "GET", url: "/api/v1/backups" }, { method: "GET", url: "/api/v1/snapshots" },
      { method: "POST", url: "/api/v1/fixes/plan" }, { method: "POST", url: "/api/v1/fixes/apply" },
      { method: "POST", url: "/api/v1/rollback" }, { method: "POST", url: "/api/v1/snapshots" },
      { method: "POST", url: "/api/v1/drift" }, { method: "POST", url: "/api/v1/export" },
    ]);
    expect(calls.every((call) => call.token === token)).toBe(true);
    expect(calls.slice(4).map((call) => JSON.parse(call.body))).toEqual([
      { revision: 7 }, { revision: 7, fix_ids: ["safe-a"] }, { revision: 7, backup_id: "backup-a" },
      { revision: 7, name: "base" }, { revision: 7, baseline: "base" }, { revision: 7, baseline: "base" },
    ]);
    for (const modeValue of ["html", "malformed", "invalid-json", "redirect", "unauthorized"]) {
      mode = modeValue;
      await expect(api.state()).rejects.toThrow(modeValue === "redirect" ? /Cannot reach/ : modeValue === "unauthorized" ? /Session/ : /JSON/);
    }
    const requestCount = calls.length;
    await expect(new APIClient(token, "https://example.invalid").state()).rejects.toThrow(/same-origin loopback/);
    await expect(new APIClient("").state()).rejects.toThrow(/launch URL/);
    expect(calls).toHaveLength(requestCount);
  } finally {
    if (previousWindow) Object.defineProperty(globalThis, "window", previousWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
});

test("captures the dashboard for local visual inspection without updating a baseline", async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("button", { name: "Inspect codex" })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("overview-local.png"), fullPage: true });
  await page.setViewportSize({ width: 768, height: 900 });
  await page.screenshot({ path: testInfo.outputPath("overview-768-local.png"), fullPage: true });
  expect(errors).toEqual([]);
});

test("dashboard source avoids HTML string injection sinks", async () => {
  const directory = new URL("../src/", import.meta.url);
  const entries = await readdir(directory).catch(() => []);
  expect(entries.length, "dashboard source exists").toBeGreaterThan(0);
  for (const name of entries.filter((name) => name.endsWith(".ts"))) {
    const source = await readFile(new URL(name, directory), "utf8");
    expect(source, fileURLToPath(new URL(name, directory))).not.toMatch(/\.(?:innerHTML|outerHTML)\s*=|insertAdjacentHTML|document\.write\s*\(/);
  }
});
