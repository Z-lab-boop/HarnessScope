import { test, expect } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile, readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { APIClient } from "../src/api.js";
import { layout } from "../src/graph.js";
import { normalize } from "../src/compare.js";
import type { ConfigNode } from "../src/types.js";

const token = "synthetic-session-token";
const fixture = JSON.parse(await readFile(new URL("./fixture-state.json", import.meta.url), "utf8"));
const origin = (scope = "PROJECT", rank = 1) => ({ source_id: "source-a", logical_path: "./config.json", scope, precedence_rank: rank, rule: "fixture precedence" });
fixture.result.analysis.graph = {
  nodes: [
    { id: "z-rule", type: "RULE", client: "opencode", display_name: "shared", adapter_confidence: "LIKELY", origins: [origin("USER")], attributes: { mode: { kind: "string", display: "review", present: true } } },
    { id: "client-a", type: "CLIENT", client: "codex", display_name: "codex", adapter_confidence: "CONFIRMED" },
    { id: "source-a", type: "SOURCE", client: "codex", display_name: "./config.json", adapter_confidence: "CONFIRMED", origins: [origin()] },
    { id: "a-rule", type: "RULE", client: "codex", display_name: "shared", adapter_confidence: "CONFIRMED", load_condition: "When workspace opens", origins: [origin("USER", 2), origin("PROJECT", 1)], attributes: { mode: { kind: "string", display: "safe", present: true }, token: { kind: "secret", display: "SECRET_CANARY", present: true, secret_category: "credential" } } },
    { id: "b-skill", type: "SKILL", client: "codex", display_name: "local-only", adapter_confidence: "UNKNOWN", origins: [origin()] },
    ...["codex", "opencode"].map((client, index) => ({ id: `same-${index}`, type: "MCP_SERVER", client, display_name: "same service", adapter_confidence: "CONFIRMED", attributes: { host: { kind: "string", display: "localhost", present: true } } })),
  ],
  edges: [
    { id: "e1", from: "client-a", to: "source-a", type: "LOADS", ruleset: "fixture", evidence: "CONFIRMED" },
    { id: "e2", from: "source-a", to: "a-rule", type: "OVERRIDES", ruleset: "fixture", evidence: "LIKELY" },
  ],
};
fixture.result.analysis.findings[0].graph_references = ["a-rule"];
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
        if (mode === "hostile") {
          data.result.analysis.clients[0].id = '<img src="https://example.invalid/leak">';
          data.result.analysis.graph.nodes[3].display_name = '<img src="https://example.invalid/leak">';
          data.result.analysis.graph.nodes[3].origins[0].logical_path = "/Users/private/PATH_CANARY";
          data.result.analysis.graph.nodes[3].origins[1].logical_path = "/home/private/PATH_CANARY";
        }
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
  const client = page.getByRole("button", { name: "Inspect codex" });
  await expect(client).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to workbench" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
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

test("reviewed Linux dashboard visual baseline", async ({ page }) => {
  test.skip(process.platform !== "linux" || process.env.HARNESSSCOPE_REVIEWED_LINUX_BASELINE !== "1", "OPEN: no reviewed Linux baseline; see docs/visual-baselines.md");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`${base}/#token=${token}`);
  await expect(page.getByRole("button", { name: "Inspect codex" })).toBeVisible();
  await expect(page).toHaveScreenshot("dashboard-linux.png", { fullPage: true, maxDiffPixelRatio: 0.03 });
});

test("reviewed Linux Graph visual baseline", async ({ page }) => {
  test.skip(process.platform !== "linux" || process.env.HARNESSSCOPE_REVIEWED_LINUX_BASELINE !== "1", "OPEN: no reviewed Linux Graph baseline; see docs/visual-baselines.md");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`${base}/?view=graph#token=${token}`);
  await expect(page.locator("[data-node-id]")).toHaveCount(7);
  await expect(page).toHaveScreenshot("graph-linux.png", { fullPage: true, maxDiffPixelRatio: 0.03 });
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

test("graph has deterministic columns, accessible nodes, evidence edges and keyboard inspector", async ({ page }) => {
  await page.goto(`${base}/?view=graph#token=${token}`);
  const nodes = page.locator("[data-node-id]");
  await expect(nodes).toHaveCount(7);
  await expect(page.locator("[data-edge-id]")).toHaveCount(2);
  await expect(page.locator('[data-edge-id="e1"]')).toHaveAttribute("stroke-dasharray", "none");
  await expect(page.locator('[data-edge-id="e2"]')).toHaveAttribute("opacity", ".65");
  await expect(page.locator('[data-edge-id="e2"]')).not.toHaveAttribute("stroke-dasharray", "none");
  await expect(page.locator('[data-node-id="client-a"]')).toHaveAttribute("transform", "translate(120 70)");
  await expect(page.locator('[data-node-id="source-a"]')).toHaveAttribute("transform", "translate(420 70)");
  const first = page.locator('[data-node-id="a-rule"]');
  await expect(first).toHaveAccessibleName(/shared.*RULE.*codex.*CONFIRMED/);
  await first.focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.locator('[data-node-id="b-skill"]')).toBeFocused();
  await page.keyboard.press("ArrowUp");
  await page.keyboard.press("Enter");
  const inspector = page.getByRole("complementary", { name: "Inspector" });
  await expect(inspector).toContainText("When workspace opens");
  await expect(inspector.locator("ol li").first()).toContainText("PROJECT");
  await expect(inspector).toContainText("[REDACTED]");
  await expect(page.locator("body")).not.toContainText("SECRET_CANARY");
});

test("graph pan and zoom clamp, reset, and release pointer capture", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(`${base}/?view=graph#token=${token}`);
  const viewport = page.locator("[data-graph-viewport]");
  await expect(viewport).toHaveAttribute("transform", "translate(0 0) scale(1)");
  await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  await expect(viewport).toHaveAttribute("transform", /scale\(1\.2\)/);
  const svg = page.getByRole("group", { name: "Configuration graph" });
  await svg.hover({ position: { x: 50, y: 220 } });
  await page.mouse.wheel(0, -500);
  await expect(viewport).not.toHaveAttribute("transform", /scale\(1\.2\)/);
  for (let i = 0; i < 15; i++) await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  await expect(viewport).toHaveAttribute("transform", /scale\(2\.5\)/);
  for (let i = 0; i < 15; i++) await page.getByRole("button", { name: "Zoom out", exact: true }).click();
  await expect(viewport).toHaveAttribute("transform", /scale\(0\.5\)/);
  await page.getByRole("button", { name: "Reset graph view" }).click();
  const box = (await svg.boundingBox())!;
  await page.mouse.move(box.x + 20, box.y + 220);
  await page.mouse.down();
  await page.mouse.move(box.x + 70, box.y + 260);
  await page.mouse.up();
  const panned = await viewport.getAttribute("transform");
  expect(panned).not.toBe("translate(0 0) scale(1)");
  await page.mouse.move(box.x + 130, box.y + 300);
  await expect(viewport).toHaveAttribute("transform", panned!);
  await page.getByRole("button", { name: "Reset graph view" }).click();
  await expect(viewport).toHaveAttribute("transform", "translate(0 0) scale(1)");
  await page.mouse.move(box.x + 20, box.y + 220);
  await page.mouse.down();
  await svg.dispatchEvent("pointercancel", { pointerId: 1 });
  await page.mouse.move(box.x + 90, box.y + 290);
  await page.mouse.up();
  await expect(viewport).toHaveAttribute("transform", "translate(0 0) scale(1)");
  expect(errors).toEqual([]);
});

test("graph filters combine without moving surviving nodes and retain input focus", async ({ page }) => {
  await page.goto(`${base}/?view=graph#token=${token}`);
  const position = await page.locator('[data-node-id="a-rule"]').getAttribute("transform");
  const search = page.getByRole("searchbox", { name: "Search graph" });
  await search.pressSequentially("shared");
  await expect(search).toBeFocused();
  await expect(page.locator("[data-node-id]")).toHaveCount(2);
  await page.getByLabel("Graph client").selectOption("codex");
  await page.getByLabel("Graph scope").selectOption("PROJECT");
  await page.getByLabel("Graph evidence").selectOption("CONFIRMED");
  await expect(page.locator("[data-node-id]")).toHaveCount(1);
  await expect(page.locator('[data-node-id="a-rule"]')).toHaveAttribute("transform", position!);
  await expect(page.locator("[data-edge-id]")).toHaveCount(0);
  await page.getByLabel("Graph evidence").selectOption("UNKNOWN");
  await expect(page.getByText("No nodes match these filters.")).toBeVisible();
});

test("findings combine all filters and focus the referenced graph node", async ({ page }) => {
  await page.goto(`${base}/?view=findings#token=${token}`);
  await page.getByRole("searchbox", { name: "Search findings" }).fill("command");
  await page.getByLabel("Finding severity").selectOption("HIGH");
  await page.getByLabel("Finding evidence").selectOption("CONFIRMED");
  await page.getByLabel("Finding client").selectOption("codex");
  await expect(page.locator(".finding-card")).toHaveCount(1);
  await page.getByLabel("Finding client").selectOption("opencode");
  await expect(page.getByText("No findings match these filters.")).toBeVisible();
  await page.getByLabel("Finding client").selectOption("codex");
  await page.getByRole("button", { name: "Focus shared in graph" }).click();
  await expect(page.getByRole("link", { name: "Graph", exact: true })).toHaveAttribute("aria-current", "page");
  await expect(page.locator('[data-node-id="a-rule"]')).toBeFocused();
  await expect(page.getByRole("complementary", { name: "Inspector" })).toContainText("When workspace opens");
});

test("filtering away the selected graph node leaves a keyboard entry point", async ({ page }) => {
  await page.goto(`${base}/?view=graph#token=${token}`);
  await page.locator('[data-node-id="a-rule"]').focus();
  await page.keyboard.press("Enter");
  await page.getByRole("searchbox", { name: "Search graph" }).fill("local-only");
  const visible = page.locator('[data-node-id="b-skill"]');
  await expect(visible).toHaveAttribute("tabindex", "0");
  await expect(page.locator('[data-node-id="a-rule"]')).toHaveCount(0);
  await page.getByRole("button", { name: "Reset graph view" }).focus();
  await page.keyboard.press("Tab");
  await expect(visible).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(visible).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("complementary", { name: "Inspector" })).toContainText("local-only");
});

test("compare normalizes by type and name and exposes presence and divergence without secrets", async ({ page }) => {
  await page.goto(`${base}/?view=compare#token=${token}`);
  await expect(page.getByRole("table", { name: "Configuration comparison: codex and opencode" })).toBeVisible();
  await expect(page.getByRole("row", { name: /RULE shared.*Divergent/ })).toBeVisible();
  await expect(page.getByRole("row", { name: /SKILL local-only.*Missing/ })).toContainText("Missing in opencode");
  await expect(page.getByRole("row", { name: /MCP_SERVER same service.*Present/ })).toBeVisible();
  await page.getByLabel("Left client").selectOption("opencode");
  await page.getByLabel("Right client").selectOption("codex");
  await expect(page.getByRole("row", { name: /SKILL local-only.*Missing/ })).toContainText("Missing in opencode");
  await expect(page.locator("body")).not.toContainText("SECRET_CANARY");
});

test("new views handle hostile text, unsafe origins, null slices and 768px geometry", async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.emulateMedia({ reducedMotion: "reduce" });
  mode = "hostile";
  await page.setViewportSize({ width: 768, height: 900 });
  await page.goto(`${base}/?view=graph#token=${token}`);
  await page.locator('[data-node-id="a-rule"]').focus();
  await page.keyboard.press("Enter");
  await expect(page.locator("body")).not.toContainText(/\/Users\/|\/home\/|PATH_CANARY|SECRET_CANARY/);
  await expect(page.locator("img")).toHaveCount(0);
  for (const view of ["Graph", "Findings", "Compare"]) {
    await page.getByRole("link", { name: view, exact: true }).click();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  mode = "empty";
  for (const view of ["graph", "findings", "compare"]) {
    await page.goto(`${base}/?view=${view}#token=${token}`);
    await expect(page.getByRole("main")).toContainText(view === "compare" ? "Two clients" : view === "graph" ? "No nodes" : "No findings");
  }
  mode = "ok";
  for (const view of ["graph", "findings", "compare"]) {
    await page.goto(`${base}/?view=${view}#token=${token}`);
    await expect(page.getByRole("main")).toHaveAttribute("aria-busy", "false");
    await page.screenshot({ path: testInfo.outputPath(`${view}-768-local.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(`${base}/?view=graph#token=${token}`);
  await expect(page.locator("[data-node-id]")).toHaveCount(7);
  await page.screenshot({ path: testInfo.outputPath("graph-local.png"), fullPage: true });
  await page.locator('[data-node-id="a-rule"]').focus();
  await page.keyboard.press("Enter");
  await page.screenshot({ path: testInfo.outputPath("inspector-local.png"), fullPage: true });
  expect(errors).toEqual([]);
});

test("normalization and layout resist input order, attribute order and duplicate-name loss", () => {
  const nodes: ConfigNode[] = structuredClone(fixture.result.analysis.graph.nodes);
  expect(layout([...nodes].reverse())).toEqual(layout(nodes));
  const source = nodes.find((node) => node.id === "source-a")!;
  expect(layout(nodes).find((node) => node.id === source.id)).toMatchObject({ x: 420, y: 70 });
  const left = nodes.find((node) => node.id === "same-0")!;
  const right = nodes.find((node) => node.id === "same-1")!;
  left.attributes = { a: { kind: "string", display: "1", present: true }, b: { kind: "secret", display: "LEFT_CANARY", present: true } };
  right.attributes = { b: { kind: "secret", display: "RIGHT_CANARY", present: true }, a: { kind: "string", display: "1", present: true } };
  expect(normalize(nodes, "codex", "opencode").find((row) => row.name === "same service")?.status).toBe("Present");
  left.attributes = { path: { kind: "string", display: "/home/alice/tool", present: true } };
  right.attributes = { path: { kind: "string", display: "/home/bob/tool", present: true } };
  expect(normalize(nodes, "codex", "opencode").find((row) => row.name === "same service")?.status).toBe("Present");
  nodes.push({ ...left, id: "same-duplicate", attributes: {} });
  expect(normalize(nodes, "codex", "opencode").find((row) => row.name === "same service")?.status).toBe("Divergent");
  nodes.push({ ...left, id: "other-type", type: "HOOK" });
  expect(normalize(nodes, "codex", "opencode").filter((row) => row.name === "same service")).toHaveLength(2);
});
