# HarnessScope Public Site and README Visual Refresh Design

Date: 2026-10-09
Status: draft for user review
Target: public GitHub Pages site for the existing HarnessScope repository

## 1. Goal

Create a visually distinctive public presence for HarnessScope that makes the project memorable before asking visitors to install anything. The approved direction is **C2 — Split Observatory**: an editorial observation-station aesthetic that places conceptual configuration-forensics photography beside genuine HarnessScope interface evidence.

The deliverable has four connected surfaces:

1. an English-first public home page with a Chinese language switch;
2. a static interactive Explore page using synthetic evidence only;
3. a Docs & Download page that reflects the repository's real release state;
4. a redesigned GitHub README header with a branded banner, product previews, and a link to the public site.

The site is for attraction and understanding, not aggressive conversion. Calls to action remain visible but secondary to the visual story.

## 2. Non-goals

The work will not:

- alter the local HarnessScope dashboard, server APIs, scan logic, or security model;
- create a hosted scanner or upload configuration to a remote service;
- add telemetry, analytics, cookies, accounts, forms, or third-party runtime scripts;
- display real user configuration, paths, credentials, names, or machine identifiers;
- fabricate a release, download count, compatibility claim, benchmark, endorsement, or product screenshot;
- present generated concept photography as a real HarnessScope interface;
- add a heavy application framework when static HTML, CSS, and TypeScript are sufficient.

## 3. Audience and message

The primary audience is developers who use multiple coding agents and want to understand configuration precedence, provenance, duplicated context, and actionable conflicts.

The central statement is:

> Every rule leaves a trail.

Supporting copy explains three actions already grounded in the product:

- **Discover** which configuration and instruction files exist.
- **Trace** where an effective rule, MCP server, hook, or skill came from.
- **Resolve** conflicts and safe fixes without exposing local configuration.

The site must keep verified and preview adapter tiers explicit. It must state that the public Explore page is synthetic and that the real product remains local and offline.

## 4. Visual system

### 4.1 Art direction

The site uses an asymmetric split between a warm editorial field and a deep technical field:

- warm mineral paper: `#EEE8DA`;
- observatory navy: `#061528`;
- ice cyan: `#59DDF5`;
- evidence amber: `#F4B84C`;
- conflict coral: `#FF667F`;
- ink: `#111418`;
- muted steel: `#89A7B8`.

The memorable motif is an evidence orbit: thin elliptical paths, nodes with distinct evidence states, and a scanning line that moves between editorial and product surfaces. The motif is functional decoration; it represents source, inheritance, comparison, and conflict rather than generic particles.

Typography pairs a local-first editorial serif stack (`Iowan Old Style`, `Baskerville`, `Palatino`, serif) with a technical monospace stack (`Berkeley Mono`, `SFMono-Regular`, `Menlo`, monospace). No font CDN is required. Large headlines use restrained optical scaling and deliberately uneven line breaks. Small labels use uppercase monospace with generous tracking.

Motion is concentrated into one entrance sequence and one evidence-orbit sequence. `prefers-reduced-motion` removes nonessential motion without hiding content.

### 4.2 Image policy

Create three original raster assets with the built-in image-generation workflow:

1. **Observatory hero** — a cinematic, empty configuration-forensics workstation with glass, cables, index cards, instrument light, and negative space; no people, logos, legible interface text, or fake HarnessScope UI.
2. **Evidence laboratory** — a close editorial photograph of translucent layers, connection traces, and tagged physical artifacts; abstract but materially believable.
3. **Local machine** — a quiet developer workstation suggesting local/offline operation without showing an identifiable person or readable private data.

Generated images are explicitly decorative concept photography. Real product sections use committed Playwright screenshots from synthetic fixtures, with captions identifying them as synthetic captures. Generated images never replace real screenshots in feature claims.

## 5. Information architecture

### 5.1 Home

The hero is a 55/45 split:

- the warm side contains the headline, concise explanation, adapter-tier chips, and restrained GitHub/Explore links;
- the dark side layers the observatory photograph with a tilted, genuine dashboard capture and an evidence-orbit overlay.

The rest of the page contains:

1. a three-step Discover / Trace / Resolve sequence connected by one orbit line;
2. an alternating editorial/product gallery using the generated photographs and real synthetic screenshots;
3. a compact client-tier matrix for Codex, Claude Code, Cursor, and OpenCode;
4. a local-first privacy statement presented as a design feature, not legal boilerplate;
5. a final GitHub and build-from-source section.

The page does not show invented usage counters, testimonials, customers, or download statistics.

### 5.2 Explore

Explore is a static guided product story, not a hosted HarnessScope server. It loads a deliberately small, committed synthetic dataset containing fictional sources, nodes, edges, client tiers, and findings.

Visitors can:

- switch among Discover, Trace, and Resolve chapters;
- select graph nodes and view a sanitized origin chain;
- filter by client and evidence tier;
- reveal one conflict and one safe-fix preview;
- compare two fictional client declarations.

The page clearly labels itself “Synthetic interactive walkthrough”. It performs no scan, upload, mutation, download generation, or network request.

### 5.3 Docs & Download

This page provides:

- the verified five-minute start from the repository;
- current compatibility tiers and boundaries;
- architecture, security, contributing, and full documentation links;
- the latest truthful installation state.

Until a tagged GitHub Release exists, the primary instruction remains **Build from source** and the Releases link is informational. The site never manufactures an archive name or “latest release” badge from assumptions.

## 6. Bilingual behavior

English is the default. A persistent `EN / 中文` switch changes all navigation, headings, body copy, captions, status labels, accessible names, and metadata that users see.

The implementation uses an explicit TypeScript translation dictionary keyed by semantic IDs. English remains present in the HTML as the no-JavaScript fallback. The selected language is stored only in local storage; no cookie or server request is used. On first visit, the site does not silently override English from browser locale.

Missing translation keys fail visibly during development and are covered by tests. Code examples and product identifiers are not translated.

## 7. Technical architecture

The site lives under `web/site/` and reuses the repository's existing Node/esbuild/Playwright toolchain:

```text
web/site/
  index.html
  explore.html
  docs.html
  src/site.ts
  src/site.css
  data/demo.json
  assets/
  tests/site.spec.ts
```

`npm run build:site` bundles TypeScript and CSS and copies HTML, assets, and synthetic data into an ignored `dist/site/` directory. All runtime URLs are relative so the output works beneath the GitHub project subpath. The existing report and dashboard builds remain unchanged.

The Explore graph uses accessible DOM/SVG authored in the repository. Generated or dataset-derived text is assigned with `textContent`; it is not inserted as raw HTML. The synthetic data schema is small and independently validated before rendering.

The public site has no dependency on the Go server and no access to local APIs. The local dashboard screenshot remains sourced from the existing synthetic Playwright fixture.

## 8. README integration

The README gains a concise visual opening:

1. a committed wide PNG capture of the finished home-page hero;
2. one sentence describing HarnessScope;
3. links to Website, Explore, Documentation, and GitHub Releases;
4. a three-image strip showing Overview, Graph/Trace, and Safe Fix preview from synthetic fixtures;
5. a short compatibility and privacy boundary before the existing five-minute start.

The technical body remains authoritative and is not replaced with marketing copy. The Chinese README receives equivalent navigation and truthful captions.

README image alt text describes the relevant visual information. Images remain readable on GitHub light and dark themes and are optimized to avoid bloating repository clones.

## 9. GitHub Pages publication

Add a dedicated Pages workflow that:

1. checks out the repository;
2. installs locked Node dependencies;
3. runs the site build and focused tests;
4. uploads only `dist/site/` as the Pages artifact;
5. deploys from `main` using GitHub's official Pages actions.

The workflow uses minimum required permissions (`contents: read`, `pages: write`, `id-token: write`) and a Pages concurrency group. It does not modify releases or repository contents.

Publication requires verifying the repository's current remote and Pages availability before enabling deployment. An existing Pages source or custom domain must be preserved and reviewed rather than silently replaced.

## 10. Accessibility and responsive behavior

- Semantic landmarks, visible focus, skip link, and keyboard-operable controls are required.
- Color is never the only carrier of evidence tier, severity, or selection state.
- Generated images have concise alt text or empty alt text when decorative.
- The graph provides a synchronized textual list and inspector for keyboard and screen-reader users.
- The split hero becomes a deliberate vertical editorial stack below 820 px.
- Screens at 360 px must have no horizontal overflow; dashboard captures may scroll inside labelled frames rather than shrink into illegibility.
- Contrast targets WCAG AA for body text and interactive controls.

## 11. Error and fallback behavior

- Without JavaScript, Home and Docs remain readable in English; Explore explains that interaction requires JavaScript.
- If an optional image fails, copy and navigation remain visible and layout dimensions remain reserved.
- Invalid synthetic demo data displays a bounded “Walkthrough unavailable” panel and does not fabricate nodes.
- Language or navigation state never blocks access to another page.
- External links are ordinary links and remain usable when GitHub Pages JavaScript is unavailable.

## 12. Testing and verification

Focused automated checks cover:

- production build from a clean checkout;
- all three pages at the repository subpath;
- zero unexpected external runtime requests;
- English default and complete Chinese toggle behavior;
- navigation, keyboard focus, reduced motion, and 360/768/1440 px layouts;
- synthetic Explore selection/filter/inspector behavior;
- truthful labels for generated photography and synthetic product evidence;
- absence of real paths, credentials, user data, placeholder copy, and invented release claims;
- link and asset integrity;
- README image presence, dimensions, alt text, and bilingual navigation.

Playwright captures approved desktop and mobile site screenshots. Existing Go, report, and dashboard suites run unchanged as the regression gate. The public URL is verified after deployment.

## 13. Delivery boundary

Completion means the site builds and tests from a clean checkout, generated and real visuals are clearly distinguished, README assets render on GitHub, and the deployed Pages URL serves all three English/Chinese pages.

It does not mean a GitHub Release was published, the marketing site scanned a real workspace, or generated imagery proves product behavior.
