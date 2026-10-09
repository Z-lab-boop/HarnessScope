# HarnessScope Public Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and publicly deploy a distinctive bilingual HarnessScope website with Home, Explore, and Docs pages, original concept photography, truthful synthetic product evidence, and a matching GitHub README refresh.

**Architecture:** Add a static site under `web/site/` that reuses the existing esbuild and Playwright toolchain without changing the Go server or local dashboard. English fallback HTML is progressively enhanced by TypeScript for Chinese translation and a synthetic Explore walkthrough; a dedicated GitHub Pages workflow deploys only the built static output.

**Tech Stack:** Static semantic HTML, TypeScript 5.9, CSS, SVG/DOM, esbuild 0.25, Playwright 1.56, Node 22, GitHub Pages official actions, built-in image generation

**Spec:** `docs/superpowers/specs/2026-10-09-harnessscope-public-site-design.md`

## Global Constraints

- Do not change the local HarnessScope dashboard, Go server APIs, scan logic, fix semantics, or release artifacts.
- The public site performs no scanning, upload, mutation, analytics, telemetry, cookie storage, or runtime third-party request.
- English is readable without JavaScript; Chinese is an explicit user-selected enhancement stored only in local storage.
- Explore uses committed fictional data only and is labelled `Synthetic interactive walkthrough` in both languages.
- Generated photography is decorative and contains no people, logos, readable UI, private data, or fake HarnessScope screenshots.
- Product screenshots come only from committed synthetic fixtures and carry truthful captions.
- Do not invent a GitHub Release, archive, download count, compatibility result, testimonial, benchmark, or customer.
- Until a verified tagged release exists, installation copy says `Build from source`; Releases remains an ordinary informational link.
- Runtime URLs are relative and must work at the GitHub project subpath.
- All body text and controls meet WCAG AA contrast; all controls are keyboard reachable; reduced motion removes nonessential motion.
- The site must have no horizontal page overflow at 360, 768, or 1440 CSS pixels.
- Preserve any existing GitHub Pages source, custom domain, or deployment configuration until explicitly inspected.

---

### Task 1: Static Build, Page Shell, and Bilingual Runtime

**Files:**
- Modify: `web/package.json`
- Modify: `web/tsconfig.json`
- Create: `web/site/build.mjs`
- Create: `web/site/test-server.mjs`
- Create: `web/site/playwright.config.ts`
- Create: `web/site/index.html`
- Create: `web/site/explore.html`
- Create: `web/site/docs.html`
- Create: `web/site/src/i18n.ts`
- Create: `web/site/src/site.ts`
- Create: `web/site/src/site.css`
- Create: `web/site/tests/shell.spec.ts`

**Interfaces:**
- Consumes: existing locked dependencies from `web/package-lock.json`.
- Produces: `npm run build:site`, `npm run test:site`, static output in `dist/site/`, `Locale = "en" | "zh-CN"`, `setLocale(locale: Locale): void`, and shared `[data-i18n]` page-shell behavior.

- [ ] **Step 1: Write the failing shell test**

Create `web/site/tests/shell.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

for (const path of ["/", "/explore.html", "/docs.html"]) {
  test(`${path} has an English-first accessible shell`, async ({ page }) => {
    const requests: string[] = [];
    page.on("request", request => requests.push(request.url()));
    await page.goto(path);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
    await expect(page.getByRole("button", { name: "切换到中文" })).toBeVisible();
    expect(requests.every(url => new URL(url).origin === new URL(page.url()).origin)).toBe(true);
  });
}

test("language selection is explicit and persists without cookies", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "切换到中文" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  expect(await page.context().cookies()).toEqual([]);
  await page.goto("/docs.html");
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
});
```

- [ ] **Step 2: Verify the shell test fails**

Run:

```bash
npm --prefix web run test:site -- --grep "English-first|language selection"
```

Expected: FAIL because `test:site`, the three pages, and the language runtime do not exist.

- [ ] **Step 3: Add build and test scripts**

Add these scripts to `web/package.json` without changing existing commands:

```json
{
  "build:site": "node site/build.mjs",
  "test:site": "npm run build:site && playwright test --config site/playwright.config.ts"
}
```

Create `web/site/build.mjs` with explicit copies and esbuild entry points:

```js
import { build } from "esbuild";
import { cp, mkdir, rm } from "node:fs/promises";

const out = new URL("../../dist/site/", import.meta.url);
await rm(out, { recursive: true, force: true });
await mkdir(new URL("assets/", out), { recursive: true });
for (const page of ["index.html", "explore.html", "docs.html"]) {
  await cp(new URL(page, import.meta.url), new URL(page, out));
}
await build({ entryPoints: [new URL("src/site.ts", import.meta.url).pathname], bundle: true, minify: true, format: "esm", outfile: new URL("assets/site.js", out).pathname });
await build({ entryPoints: [new URL("src/site.css", import.meta.url).pathname], bundle: true, minify: true, outfile: new URL("assets/site.css", out).pathname });
for (const dir of ["assets", "data"]) {
  await cp(new URL(`${dir}/`, import.meta.url), new URL(`${dir}/`, out), { recursive: true }).catch(error => {
    if (error.code !== "ENOENT") throw error;
  });
}
```

Create a Playwright config whose `webServer.command` is `node site/test-server.mjs ../../dist/site` and whose base URL is `http://127.0.0.1:4177`. The static server must mount identical content at `/` and `/HarnessScope/`, reject `..`, serve correct content types, and bind only `127.0.0.1`. Add a test that loads `/HarnessScope/explore.html` and confirms its CSS, JavaScript, data, and images all return 200 from the same subpath.

- [ ] **Step 4: Implement the semantic shell and i18n contract**

Each HTML file must contain English fallback content, relative asset URLs, a skip link, header/nav/main/footer, and:

```html
<button class="language-switch" type="button" data-language-switch aria-label="切换到中文">中文</button>
<script type="module" src="./assets/site.js"></script>
```

Define `web/site/src/i18n.ts`:

```ts
export type Locale = "en" | "zh-CN";
export type CopyKey = "nav.home" | "nav.explore" | "nav.docs" | "language.switch";

export const copy: Record<Locale, Record<CopyKey, string>> = {
  en: { "nav.home": "Home", "nav.explore": "Explore", "nav.docs": "Docs & Download", "language.switch": "切换到中文" },
  "zh-CN": { "nav.home": "首页", "nav.explore": "交互体验", "nav.docs": "文档与下载", "language.switch": "Switch to English" },
};
```

In `site.ts`, accept only `en` and `zh-CN` from local storage, update `document.documentElement.lang`, replace text for every declared `data-i18n` key, update the switch label/accessibility name, and persist only the explicit button choice. Unknown keys throw during development and render their English fallback in the production bundle.

- [ ] **Step 5: Add the shared C2 visual foundation**

In `site.css`, define the approved palette and type system:

```css
:root { --paper:#eee8da; --navy:#061528; --cyan:#59ddf5; --amber:#f4b84c; --coral:#ff667f; --ink:#111418; --steel:#89a7b8; }
body { margin:0; color:var(--ink); background:var(--paper); font-family:"Iowan Old Style",Baskerville,Palatino,serif; }
.technical, nav, button { font-family:"Berkeley Mono","SFMono-Regular",Menlo,monospace; }
:focus-visible { outline:3px solid var(--coral); outline-offset:4px; }
@media (prefers-reduced-motion: reduce) { *,*::before,*::after { animation-duration:.001ms!important; animation-iteration-count:1!important; scroll-behavior:auto!important; } }
```

Use a 55/45 split above 820 px and a vertical stack below it. Do not add a purple gradient, generic card grid, or externally hosted font.

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
npm --prefix web run build:site
npm --prefix web run test:site -- --grep "English-first|language selection"
npx --prefix web tsc --noEmit -p web/tsconfig.json
git diff --check
```

Expected: shell tests pass, the build has three pages, and TypeScript/whitespace checks are clean.

Commit:

```bash
git add web/package.json web/tsconfig.json web/site
git commit -m "feat: add bilingual public site shell"
```

### Task 2: Original Image Assets and the Split Observatory Home Page

**Files:**
- Create: `web/site/assets/generated/observatory-hero.png`
- Create: `web/site/assets/generated/evidence-lab.png`
- Create: `web/site/assets/generated/local-machine.png`
- Create: `web/site/assets/generated/README.md`
- Create: `web/site/assets/product/dashboard-overview.png`
- Modify: `web/site/index.html`
- Create: `web/site/src/home.ts`
- Modify: `web/site/src/site.ts`
- Modify: `web/site/src/site.css`
- Create: `web/site/tests/home.spec.ts`

**Interfaces:**
- Consumes: Task 1 shell/i18n and `docs/assets/dashboard-overview.png` as the truthful synthetic product capture.
- Produces: `initHome(): void`, three generated decorative photographs, one labelled product screenshot, and a responsive C2 home page.

- [ ] **Step 1: Write failing home-page assertions**

Create `home.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

test("home tells the product story with truthful image roles", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Every rule leaves a trail." })).toBeVisible();
  await expect(page.getByText("Synthetic dashboard capture", { exact: true })).toBeVisible();
  await expect(page.getByText("Concept photograph", { exact: true }).first()).toBeVisible();
  for (const label of ["Discover", "Trace", "Resolve"]) await expect(page.getByRole("heading", { name: label })).toBeVisible();
  await expect(page.getByText("VERIFIED", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("PREVIEW", { exact: true }).first()).toBeVisible();
});

for (const width of [360, 768, 1440]) test(`home has no overflow at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
```

- [ ] **Step 2: Run the test and confirm the missing-content failure**

Run `npm --prefix web run test:site -- --grep "product story|overflow"`.

Expected: FAIL because the approved home content and assets are absent.

- [ ] **Step 3: Generate the three decorative images**

Use the built-in image generation tool once per asset. After each result, copy the selected generated file from its reported `$CODEX_HOME/generated_images/...` path into `web/site/assets/generated/`; never leave a project reference pointing outside the repository. Confirm the destination does not already exist before copying. Use these exact content prompts, adding only execution metadata required by the tool:

```text
observatory-hero.png — photorealistic-natural; wide editorial photograph of an empty configuration-forensics observatory workstation, smoked glass, dark navy instrument panels without readable UI, translucent cable paths, cyan instrument light, one amber evidence marker, warm mineral paper notes with no legible writing, dramatic negative space, refined technical magazine photography; no person, no logo, no watermark, no readable text, no fake software interface.

evidence-lab.png — photorealistic-natural; macro editorial photograph of layered translucent sheets, fine connection traces, metal clips and one coral conflict marker on a dark evidence table, cyan edge lighting, tactile materials, shallow depth of field; no person, no logo, no watermark, no readable text, no computer interface.

local-machine.png — photorealistic-natural; quiet developer workstation in a dim observatory room, one closed local machine beside a disconnected network cable and physical notebook, navy and warm paper palette, cyan status light, architectural shadows; no person, no brand, no watermark, no readable screen or text.
```

Record generation date, built-in tool mode, final prompt, filename, decorative role, and “not a product screenshot” in `assets/generated/README.md`. Inspect every image for accidental text, logos, faces, or apparent private data; regenerate a failing image rather than editing around the defect.

- [ ] **Step 4: Implement the split hero and evidence story**

Copy `docs/assets/dashboard-overview.png` byte-for-byte to `web/site/assets/product/dashboard-overview.png` and label it in visible copy as a Playwright synthetic fixture capture.

Build the Home page with semantic sections and this hierarchy:

```html
<section class="hero-observatory" aria-labelledby="home-title">
  <div class="hero-copy"><p class="eyebrow">Configuration forensics · local by design</p><h1 id="home-title">Every rule leaves a trail.</h1></div>
  <figure class="hero-evidence"><img src="./assets/generated/observatory-hero.png" alt=""><img class="product-capture" src="./assets/product/dashboard-overview.png" alt="HarnessScope synthetic dashboard overview"><figcaption>Synthetic dashboard capture · Concept photograph used as atmosphere</figcaption></figure>
</section>
```

Add an SVG orbit whose nodes are real buttons linking to Discover, Trace, and Resolve sections. `initHome()` observes sections and updates only `aria-current` and CSS state. It must not depend on scroll animation for access to content.

Add the four-client tier matrix and local-first privacy section using claims already present in README. Avoid numbers except verified version strings and feature facts.

- [ ] **Step 5: Validate images, responsiveness, and reduced motion**

Extend `home.spec.ts` to assert that every image loads with nonzero natural dimensions, decorative images have empty alt text, the real dashboard has descriptive alt text, the orbit is keyboard operable, and animation names are `none` under reduced motion.

Run:

```bash
npm --prefix web run test:site -- --grep "home|overflow|motion|image"
git diff --check
```

- [ ] **Step 6: Commit**

```bash
git add web/site/assets web/site/index.html web/site/src/home.ts web/site/src/site.ts web/site/src/site.css web/site/tests/home.spec.ts
git commit -m "feat: create split observatory home page"
```

### Task 3: Synthetic Interactive Explore Walkthrough

**Files:**
- Create: `web/site/data/demo.json`
- Create: `web/site/src/demo-schema.ts`
- Create: `web/site/src/explore.ts`
- Modify: `web/site/src/site.ts`
- Modify: `web/site/src/site.css`
- Modify: `web/site/explore.html`
- Create: `web/site/tests/explore.spec.ts`

**Interfaces:**
- Consumes: Task 1 shell/i18n.
- Produces: `parseDemo(input: unknown): Demo`, `initExplore(): Promise<void>`, client/evidence filtering, node inspector, fictional conflict, and safe-fix preview.

- [ ] **Step 1: Add failing Explore behavior tests**

```ts
test("Explore is explicitly synthetic and supports keyboard inspection", async ({ page }) => {
  await page.goto("/explore.html");
  await expect(page.getByText("Synthetic interactive walkthrough", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Inspect project instructions" }).focus();
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
```

- [ ] **Step 2: Verify tests fail**

Run `npm --prefix web run test:site -- --grep "Explore"`.

Expected: FAIL because no demo schema, data, or interactions exist.

- [ ] **Step 3: Define and validate the small demo schema**

Define exact types in `demo-schema.ts`:

```ts
export type Evidence = "CONFIRMED" | "LIKELY" | "UNKNOWN";
export interface DemoNode { id:string; label:string; kind:"client"|"source"|"rule"|"mcp"; client:"codex"|"claude"|"cursor"|"opencode"; evidence:Evidence; origin:string; }
export interface DemoEdge { from:string; to:string; relation:"loads"|"overrides"|"duplicates"; evidence:Evidence; }
export interface Demo { schema_version:"1"; nodes:DemoNode[]; edges:DemoEdge[]; conflict:{ title:string; node_ids:string[]; }; fix_preview:{ title:string; risk:"SAFE"; patch:string[]; }; }
```

`parseDemo` rejects unknown top-level fields, duplicate node IDs, missing edge endpoints, non-allowlisted enums, absolute paths, home-directory shorthand, URLs, email addresses, and credential-shaped strings. It returns a defensive copy sorted by node ID and edge tuple.

Create `demo.json` using only fictional relative paths such as `./AGENTS.md`, `./.codex/config.toml`, and `./.claude/settings.json`. Do not copy the larger dashboard fixture wholesale.

- [ ] **Step 4: Implement the accessible SVG/DOM walkthrough**

Render the graph with SVG lines/circles plus an adjacent semantic button list. Selecting either representation updates the same inspector. Use `textContent`, `setAttribute`, and created elements only; do not place demo values into `innerHTML`.

Provide client and evidence filters, Discover/Trace/Resolve chapter tabs, one conflict region, and one safe-fix preview. “Apply” is not present. The preview closes or resets without changing data.

On fetch/schema failure, show `Walkthrough unavailable` and a link to the repository demo instructions; do not render partial or invented nodes.

- [ ] **Step 5: Add malformed-data and privacy tests**

Unit-test `parseDemo` through Playwright-imported browser code or a Node test entry for duplicate IDs, absolute paths, unknown evidence, and credential canaries. Assert that page text contains none of `/Users/`, `/home/`, `sk-`, `@example.com`, or session tokens.

Run:

```bash
npm --prefix web run test:site -- --grep "Explore|demo schema|privacy"
npx --prefix web tsc --noEmit -p web/tsconfig.json
git diff --check
```

- [ ] **Step 6: Commit**

```bash
git add web/site/data web/site/explore.html web/site/src/demo-schema.ts web/site/src/explore.ts web/site/src/site.ts web/site/src/site.css web/site/tests/explore.spec.ts
git commit -m "feat: add synthetic evidence walkthrough"
```

### Task 4: Docs Page and Complete English/Chinese Content

**Files:**
- Create: `web/site/src/content.ts`
- Modify: `web/site/src/i18n.ts`
- Modify: `web/site/src/site.ts`
- Modify: `web/site/docs.html`
- Modify: `web/site/index.html`
- Modify: `web/site/explore.html`
- Create: `web/site/tests/content.spec.ts`

**Interfaces:**
- Consumes: Tasks 1–3 page elements and current README compatibility/safety facts.
- Produces: complete `Record<Locale, Record<ContentKey,string>>`, truthful Docs page, and bilingual visible copy across all pages.

- [ ] **Step 1: Write failing content-completeness tests**

```ts
test("every declared translation renders in both languages", async ({ page }) => {
  for (const path of ["/", "/explore.html", "/docs.html"]) {
    await page.goto(path);
    expect(await page.locator("[data-i18n]").count()).toBeGreaterThan(0);
    expect(await page.locator("[data-i18n]:text('undefined'), [data-i18n]:text('missing')").count()).toBe(0);
    await page.getByRole("button", { name: "切换到中文" }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    expect((await page.locator("main").innerText()).trim().length).toBeGreaterThan(200);
  }
});

test("Docs states the truthful release boundary", async ({ page }) => {
  await page.goto("/docs.html");
  await expect(page.getByRole("heading", { name: "Build from source" })).toBeVisible();
  await expect(page.getByText("Download v0.2", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "GitHub Releases" })).toHaveAttribute("href", /releases/);
});
```

- [ ] **Step 2: Verify the content tests fail**

Run `npm --prefix web run test:site -- --grep "translation|release boundary"`.

Expected: FAIL because full copy and Docs content are not yet wired.

- [ ] **Step 3: Implement exhaustive keyed content**

Define `ContentKey` as `keyof typeof english` and require `const chinese: Record<ContentKey,string>`. Include every visible headline, body paragraph, caption, status, accessible name, and error string. Retain product names and code literals unchanged.

The Docs page must include the current commands:

```sh
go build -trimpath -o bin/hscope ./cmd/hscope
./bin/hscope serve . --port 0 --open
```

It must link relatively or to repository-root GitHub URLs for architecture, security, contributing, releases, and the full bilingual READMEs. It must reproduce VERIFIED/PREVIEW distinctions and local/offline boundaries without strengthening them.

- [ ] **Step 4: Verify semantics, language, and no-JavaScript fallback**

Add a Playwright JavaScript-disabled project for Home and Docs only. Assert their English headings, build command, navigation links, image captions, and privacy boundary remain visible. Explore must show a static message explaining that interaction requires JavaScript.

Run:

```bash
npm --prefix web run test:site -- --grep "translation|release boundary|JavaScript"
npx --prefix web tsc --noEmit -p web/tsconfig.json
git diff --check
```

- [ ] **Step 5: Commit**

```bash
git add web/site/src/content.ts web/site/src/i18n.ts web/site/src/site.ts web/site/index.html web/site/explore.html web/site/docs.html web/site/tests/content.spec.ts
git commit -m "feat: complete bilingual site content"
```

### Task 5: README Visual Refresh and Reproducible Site Captures

**Files:**
- Create: `web/site/capture.mjs`
- Create: `docs/assets/site-hero.png`
- Create: `docs/assets/site-explore.png`
- Create: `docs/assets/site-docs.png`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Create: `scripts/check-site-assets.mjs`
- Modify: `scripts/check-repository.mjs`
- Create: `web/site/tests/capture.spec.ts`

**Interfaces:**
- Consumes: finished static site from Tasks 1–4.
- Produces: `npm run capture:site`, three deterministic GitHub-ready PNGs, bilingual README navigation, and repository asset checks.

- [ ] **Step 1: Write failing capture and README checks**

Create `scripts/check-site-assets.mjs` that requires the three PNG signatures, bounds each file to 2 MiB, and verifies both READMEs reference `docs/assets/site-hero.png` with nonempty alt text plus Website/Explore/Docs links. The script must emit the exact failing filename and condition.

Add a failing test that calls the script and expects success after assets exist.

Run `node scripts/check-site-assets.mjs`.

Expected: FAIL because the assets and README links do not exist.

- [ ] **Step 2: Add deterministic capture tooling**

Add `capture:site` to `web/package.json` and create `capture.mjs`. It starts the built site server, uses the installed Chromium, sets viewport `1600×1000`, forces English and reduced motion, waits for `document.fonts.ready` plus loaded images, and writes:

```text
docs/assets/site-hero.png       Home hero locator only
docs/assets/site-explore.png    Explore walkthrough main panel
docs/assets/site-docs.png       Docs header plus start panel
```

Use stable locators (`data-capture="hero"`, `explore`, `docs`) rather than pixel coordinates. Do not rewrite approved images during ordinary `npm run build:site`; capture is an explicit maintenance action.

- [ ] **Step 3: Refresh both README openings**

Keep existing technical sections intact. Replace only the opening with this structure in both languages:

```markdown
![HarnessScope Split Observatory public site](docs/assets/site-hero.png)

# HarnessScope

Local configuration forensics for coding agents.

[Website](https://z-lab-boop.github.io/HarnessScope/) · [Explore](https://z-lab-boop.github.io/HarnessScope/explore.html) · [Docs](https://z-lab-boop.github.io/HarnessScope/docs.html) · [Releases](https://github.com/Z-lab-boop/HarnessScope/releases)

| Discover | Trace | Resolve |
| --- | --- | --- |
| Find configuration sources | Follow provenance and precedence | Review conflicts and SAFE fixes |
```

These URLs were derived from the verified `origin` remote `https://github.com/Z-lab-boop/HarnessScope.git`. Re-check that remote before editing the READMEs; if it differs, stop and update the design rather than silently publishing links for another repository. Add the Explore and Docs images below the existing local-control-center explanation with truthful synthetic/generated captions.

- [ ] **Step 4: Capture and verify assets**

Run:

```bash
npm --prefix web run build:site
npm --prefix web run capture:site
node scripts/check-site-assets.mjs
npm --prefix web run test:site -- --grep "capture|README"
git diff --check
```

Manually inspect all three PNGs at original resolution for clipping, illegible text, false UI, accidental identifiers, and inconsistent language.

- [ ] **Step 5: Commit**

```bash
git add web/package.json web/site/capture.mjs web/site/tests/capture.spec.ts docs/assets/site-hero.png docs/assets/site-explore.png docs/assets/site-docs.png README.md README.zh-CN.md scripts/check-site-assets.mjs scripts/check-repository.mjs
git commit -m "docs: refresh HarnessScope visual identity"
```

### Task 6: CI, GitHub Pages, Full Verification, and Public Deployment

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/pages.yml`
- Modify: `.gitignore`
- Modify: `docs/release-gates.md`
- Verify: all files changed by Tasks 1–5

**Interfaces:**
- Consumes: `npm run build:site`, `npm run test:site`, `node scripts/check-site-assets.mjs`, and `dist/site/`.
- Produces: CI-gated Pages artifact, public Pages URL, clean source checkout, and verified README links.

- [ ] **Step 1: Add the site to ordinary CI**

After the existing web build/typecheck steps in `ci.yml`, add:

```yaml
      - run: npm --prefix web run build:site
      - run: npm --prefix web run test:site
      - run: node scripts/check-site-assets.mjs
```

Keep the existing Go, dashboard, report, demo, safety, and release gates unchanged.

- [ ] **Step 2: Create the Pages workflow**

Create `.github/workflows/pages.yml`:

```yaml
name: Pages
on:
  push:
    branches: [main]
  workflow_dispatch:
permissions:
  contents: read
  pages: write
  id-token: write
concurrency:
  group: pages
  cancel-in-progress: true
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm --prefix web ci
      - run: npm --prefix web run build:site
      - run: npm --prefix web run test:site
      - run: node scripts/check-site-assets.mjs
      - uses: actions/configure-pages@v5
      - uses: actions/upload-pages-artifact@v3
        with:
          path: dist/site
  deploy:
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    needs: build
    steps:
      - name: Deploy
        id: deployment
        uses: actions/deploy-pages@v4
```

- [ ] **Step 3: Update repository hygiene and release documentation**

Add `/.superpowers/` and `/dist/site/` to `.gitignore`; keep the approved design/spec and plan because they live under `docs/`, not the ignored root brainstorm directory. Document the public-site build, tests, capture command, generated-image boundary, and Pages deployment gate in `docs/release-gates.md`.

- [ ] **Step 4: Run the complete local gate**

Run:

```bash
npm --prefix web ci
npm --prefix web run build
npm --prefix web run build:site
npx --prefix web tsc --noEmit -p web/tsconfig.json
npm --prefix web run test:visual
npm --prefix web run test:dashboard
npm --prefix web run test:site
node scripts/check-site-assets.mjs
node scripts/check-repository.mjs
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Expected: every command passes; generated dashboard/report assets match source; no site request leaves the local test origin.

- [ ] **Step 5: Run privacy, credential, and claim scans**

Run:

```bash
rg -n "TBD|TODO|FIXME|PLACEHOLDER|sk-[A-Za-z0-9]|BEGIN (RSA|OPENSSH|EC) PRIVATE KEY" web/site docs/assets README.md README.zh-CN.md .github/workflows/pages.yml
rg -n "/Users/|/home/|@[A-Za-z0-9.-]+\.[A-Za-z]{2,}" web/site/data web/site/src docs/assets
rg -n "downloads|customers|trusted by|guaranteed|fully compatible|official release" web/site README.md README.zh-CN.md
```

Expected: no placeholder, credential, private path, personal email, or invented marketing claim. Intentional boundary prose must be reviewed in context rather than suppressed.

- [ ] **Step 6: Commit the deployment configuration**

```bash
git add .github/workflows/ci.yml .github/workflows/pages.yml .gitignore docs/release-gates.md
git commit -m "ci: publish HarnessScope site with Pages"
```

- [ ] **Step 7: Verify Pages state before changing it**

Read the exact owner/repository from `git remote get-url origin`. Check:

```bash
gh auth status
gh api repos/Z-lab-boop/HarnessScope/pages
```

If Pages already exists, record its `build_type`, source, URL, and custom-domain state. Stop instead of replacing a non-workflow source or existing custom domain silently. If the endpoint returns 404 and the repository supports Pages, enable workflow builds with:

```bash
gh api --method POST repos/Z-lab-boop/HarnessScope/pages -f build_type=workflow
```

- [ ] **Step 8: Publish through the approved branch integration path**

Use the `finishing-a-development-branch` skill. The user has authorized public publication, but branch integration still follows the skill's explicit merge/PR choice. Once `main` contains the reviewed commits, push it and watch the Pages workflow to completion:

```bash
git push origin main
run_id=$(gh run list --workflow Pages --branch main --limit 1 --json databaseId --jq '.[0].databaseId')
gh run watch "$run_id" --exit-status
```

Do not force-push. If remote `main` moved, fetch and inspect before continuing.

- [ ] **Step 9: Verify the public result**

Open the exact `page_url` returned by the deployment and verify Home, Explore, and Docs, English/Chinese switching, assets, responsive layout, and README links. Check headers and HTML for accidental private paths or unexpanded placeholders. Record the deployed URL, workflow run, commit hash, local test counts, and the boundary that Explore is synthetic and generated images are conceptual.

Expected: all three public pages return success, README links resolve, the site contains no external runtime resources, and the working tree is clean.
