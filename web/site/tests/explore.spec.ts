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

test("Explore synchronizes SVG and list selection, preserves focus and traces origins", async ({ page }) => {
  await page.goto("/explore.html");
  const node = page.getByRole("button", { name: "Select in graph: project instructions · CONFIRMED", exact: true });
  await node.focus();
  await page.keyboard.press("Space");
  await expect(node).toBeFocused();
  await expect(node).toHaveAttribute("aria-pressed", "true");
  const inspector = page.getByRole("complementary", { name: "Evidence inspector" });
  await expect(inspector).toContainText("./.codex/config.toml → loads · CONFIRMED");
  await expect(page.getByRole("button", { name: "Inspect project instructions", exact: true })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Inspect Claude demo-tools", exact: true }).click();
  await expect(node).toHaveAttribute("aria-pressed", "false");
  await expect(inspector).toContainText("duplicates: Codex demo-tools");
});

test("Explore combines filters, clears stale inspection and handles empty results", async ({ page }) => {
  await page.goto("/explore.html");
  await page.getByRole("combobox", { name: "Client", exact: true }).selectOption("cursor");
  await expect(page.locator("[data-explore-nodes] button")).toHaveCount(2);
  await expect(page.getByRole("status")).toContainText("2 / 9");
  await expect(page.getByRole("complementary", { name: "Evidence inspector" })).toContainText("Select a declaration");
  await page.getByRole("combobox", { name: "Evidence", exact: true }).selectOption("CONFIRMED");
  await expect(page.locator("[data-explore-nodes] button")).toHaveCount(0);
  await expect(page.getByText("No declarations match these filters.")).toBeVisible();
  await page.getByRole("button", { name: "Reset view" }).click();
  await expect(page.locator("[data-explore-nodes] button")).toHaveCount(9);
  await expect(page.locator(".graph-edge")).toHaveCount(6);
});

test("Explore chapter tabs support arrow, Home and End keyboard controls", async ({ page }) => {
  await page.goto("/explore.html");
  await page.getByRole("tab", { name: "Discover", exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Trace", exact: true })).toBeFocused();
  await expect(page.getByRole("tabpanel")).toContainText("Follow declared links");
  await page.keyboard.press("End");
  await expect(page.getByRole("tab", { name: "Resolve", exact: true })).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("Home");
  await expect(page.getByRole("tab", { name: "Discover", exact: true })).toBeFocused();
});

test("Explore closes and resets previews without changing source evidence", async ({ page, request }) => {
  const before = await (await request.get("/data/demo.json")).json();
  await page.goto("/explore.html");
  await page.getByRole("button", { name: "Show conflicts" }).click();
  const comparison = page.locator("[data-explore-comparison]");
  await expect(comparison).toContainText("./.codex/config.toml");
  await expect(comparison).toContainText("./.claude/settings.json");
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  await expect(page.locator("[data-explore-patch]")).toContainText("- demo-tools: duplicate declaration");
  await page.getByRole("button", { name: "Close preview" }).click();
  await expect(page.getByRole("button", { name: "Preview safe fix" })).toBeFocused();
  await expect(page.locator("#fix-preview")).toBeHidden();
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  await page.getByRole("button", { name: "Reset view" }).click();
  await expect(page.locator("#conflict-evidence")).toBeHidden();
  await expect(page.locator("#fix-preview")).toBeHidden();
  await expect(page.getByRole("button", { name: /^Apply/ })).toHaveCount(0);
  expect(await (await request.get("/data/demo.json")).json()).toEqual(before);
});

test("Explore translates static and dynamic UI without losing filters or selection", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("/explore.html");
  await page.getByRole("combobox", { name: "Client", exact: true }).selectOption("codex");
  await page.getByRole("button", { name: "Inspect project instructions", exact: true }).click();
  await page.getByRole("button", { name: "Show conflicts" }).click();
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  const english = await page.locator('main [data-i18n], main [data-i18n-aria]').evaluateAll(elements => elements.map(element => element.textContent + element.getAttribute("aria-label")));
  await page.getByRole("button", { name: "切换到中文" }).click();
  const chinese = await page.locator('main [data-i18n], main [data-i18n-aria]').evaluateAll(elements => elements.map(element => element.textContent + element.getAttribute("aria-label")));
  expect(chinese.every((value, index) => value !== english[index])).toBe(true);
  await expect(page.getByRole("complementary", { name: "证据检查器" })).toContainText("项目指令");
  await expect(page.getByRole("button", { name: "检查 项目指令", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("status")).toHaveText("已打开安全修复预览 · 未做任何更改");
  await expect(page.getByRole("combobox", { name: "客户端", exact: true })).toHaveValue("codex");
  await page.getByRole("button", { name: "Switch to English" }).click();
  await expect(page.getByRole("heading", { name: "Synthetic interactive walkthrough", exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

for (const width of [360, 768, 1440]) test(`Explore fits ${width}px with both languages and preview open`, async ({ page }, testInfo) => {
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/HarnessScope/explore.html");
  await page.getByRole("button", { name: "Show conflicts" }).click();
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  for (const name of ["切换到中文", "Switch to English"]) {
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByRole("button", { name }).click();
  }
  await page.screenshot({ path: testInfo.outputPath(`explore-${width}.png`), fullPage: true });
});

test("Explore privacy keeps all requests read-only, local and subpath-safe", async ({ page }) => {
  const requests: { url: string; method: string }[] = [];
  page.on("request", request => requests.push({ url: request.url(), method: request.method() }));
  await page.goto("/HarnessScope/explore.html");
  await page.getByRole("button", { name: "Show conflicts" }).click();
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  const body = await page.locator("body").innerText();
  for (const canary of ["/Users/", "/home/", "sk-", "@example.com", "session_token", "Bearer "]) expect(body).not.toContain(canary);
  expect(requests.some(request => request.url.endsWith("/HarnessScope/data/demo.json"))).toBe(true);
  expect(requests.every(request => request.method === "GET" && new URL(request.url).origin === new URL(page.url()).origin && new URL(request.url).pathname.startsWith("/HarnessScope/"))).toBe(true);
  expect(await page.context().cookies()).toEqual([]);
});

for (const failure of ["network", "status", "json", "schema"]) test(`Explore handles ${failure} failure without partial data`, async ({ page }) => {
  await page.route("**/data/demo.json", route => failure === "network" ? route.abort() : route.fulfill({ status: failure === "status" ? 503 : 200, contentType: "application/json", body: failure === "json" ? "{" : JSON.stringify({ ...fixture(), nodes: [...fixture().nodes, fixture().nodes[0]] }) }));
  await page.goto("/explore.html");
  await expect(page.getByRole("heading", { name: "Walkthrough unavailable" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Read the repository demo instructions ↗" })).toHaveAttribute("href", "https://github.com/Z-lab-boop/HarnessScope/blob/main/demo/conflicted-workspace/EXPECTED.md");
  await expect(page.locator("[data-node-id]")).toHaveCount(0);
  await expect(page.locator("[data-explore-workbench]")).toBeHidden();
  await page.getByRole("button", { name: "切换到中文" }).click();
  await expect(page.getByRole("heading", { name: "交互演示暂不可用" })).toBeVisible();
});

test("demo schema makes a defensive, deterministically sorted copy", () => {
  const input = fixture();
  input.nodes.reverse(); input.edges.reverse();
  const result = parseDemo(input);
  expect(result).toEqual(parseDemo(fixture()));
  input.nodes[0].label = "changed"; input.fix_preview.patch[0] = "changed";
  expect(result.nodes.some(node => node.label === "changed")).toBe(false);
  expect(result.fix_preview.patch[0]).not.toBe("changed");
  result.nodes[0].label = "result changed";
  expect(fixture().nodes.some((node: { label: string }) => node.label === "result changed")).toBe(false);
});

test("demo schema rejects unknown fields, unsupported enums and invalid references", () => {
  const mutations: ((demo: ReturnType<typeof fixture>) => void)[] = [
    demo => { demo.unknown = true; }, demo => { demo.nodes.push(demo.nodes[0]); },
    demo => { demo.edges[0].to = "missing"; }, demo => { demo.nodes[0].evidence = "CERTAIN"; },
    demo => { demo.nodes[0].client = "other"; }, demo => { demo.nodes[0].kind = "secret"; },
    demo => { demo.edges[0].relation = "executes"; }, demo => { demo.edges[0].evidence = "CERTAIN"; },
    demo => { demo.conflict.node_ids[0] = "missing"; }, demo => { demo.fix_preview.risk = "UNSAFE"; },
    demo => { demo.schema_version = "2"; }, demo => { demo.nodes = []; }, demo => { demo.nodes[0].unexpected = "extra"; },
  ];
  for (const mutate of mutations) { const demo = fixture(); mutate(demo); expect(() => parseDemo(demo)).toThrow("Invalid synthetic demo"); }
  for (const input of [null, [], 1, "demo", {}]) expect(() => parseDemo(input)).toThrow();
});

test("demo schema privacy rejects absolute paths, emails, URLs and credential canaries everywhere", () => {
  const canaries = ["/Users/fictional/config", "/home/fictional/config", "/etc/config", "C:\\fictional\\config", "\\\\machine\\share", "\\rooted\\config", "~/config", "~fictional/config", "https://invalid.test", "file:///config", "data:text/plain,fictional", "mailto:fictional", "person@example.com", "sk-demo-canary", "ghp_canarycanarycanary", "github_pat_canary", "AKIA1234567890123456", "Bearer fictional-token", "session_token=fictional", "api_key=fictional", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJmaWN0aW9uIn0.signature"];
  for (const canary of canaries) {
    for (const field of ["label", "origin", "title", "patch"]) {
      const demo = fixture();
      if (field === "label" || field === "origin") demo.nodes[0][field] = canary;
      if (field === "title") demo.conflict.title = canary;
      if (field === "patch") demo.fix_preview.patch[0] = `+ ${canary}`;
      expect(() => parseDemo(demo), `${field}: ${canary}`).toThrow();
    }
  }
  for (const origin of ["./../config", "./nested/./config", "./nested/../../config", "./config?query", "./config%20name"]) {
    const demo = fixture(); demo.nodes[0].origin = origin; expect(() => parseDemo(demo)).toThrow();
  }
});

test("demo schema privacy rejects quoted credential assignments in every display field", () => {
  const assignments = [
    '"api_key": "fictional-private-value"',
    '"access_token": "fictional-private-value"',
    "'api_key': 'fictional-private-value'",
    "'access_token': 'fictional-private-value'",
    '{"API_KEY": "fictional-private-value"}',
    "{'AcCeSs-ToKeN' : 'fictional-private-value'}",
    '"session_token" = "fictional-private-value"',
    "'PASSWORD' = 'fictional-private-value'",
    '"secret"\t:\t"fictional-private-value"',
  ];
  for (const assignment of assignments) {
    for (const field of ["label", "conflictTitle", "fixTitle", "patch"]) {
      const demo = fixture();
      if (field === "label") demo.nodes[0].label = assignment;
      if (field === "conflictTitle") demo.conflict.title = assignment;
      if (field === "fixTitle") demo.fix_preview.title = assignment;
      if (field === "patch") demo.fix_preview.patch[0] = `+ ${assignment}`;
      expect(() => parseDemo(demo), `${field}: ${assignment}`).toThrow("Invalid synthetic demo");
    }
  }
});

test("Explore uses validated dataset text safely without interpreting HTML", async ({ page }) => {
  const demo = fixture();
  demo.nodes[0].label = '<img src=x onerror="alert(1)">';
  demo.conflict.title = "Fictional test finding";
  demo.fix_preview.title = "Fictional test preview";
  await page.route("**/data/demo.json", route => route.fulfill({ contentType: "application/json", body: JSON.stringify(demo) }));
  await page.goto("/explore.html");
  await expect(page.getByRole("button", { name: `Inspect ${demo.nodes[0].label}`, exact: true })).toBeVisible();
  await expect(page.locator("main img")).toHaveCount(0);
  await page.getByRole("button", { name: "Show conflicts" }).click();
  await expect(page.getByRole("heading", { name: "Fictional test finding" })).toBeVisible();
  await page.getByRole("button", { name: "Preview safe fix" }).click();
  await expect(page.getByRole("heading", { name: "Fictional test preview" })).toBeVisible();
});
