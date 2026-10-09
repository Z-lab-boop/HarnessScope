# v0.2 release gates

Local candidates are supported for darwin/linux × amd64/arm64. Functional CI runs on macOS and Linux; release publication also requires the separate reviewed-Linux visual job. That visual gate is **OPEN** because reviewed report, Overview and Graph baselines are absent. Ordinary push/PR CI shows a non-blocking OPEN warning; reusable publication CI sets `strict_release: true` and fails closed. When all baselines exist, ordinary CI compares them too. Cross-compilation and macOS browser success do not close this gate. No repository creation, push, tag or publication is performed by local scripts.

Archives use the explicit `scripts/release-files.txt` allowlist; internal design/plan directories are excluded. Each archive is checked for exact regular-file membership, freshly extracted, and scanned for personal paths and synthetic canaries. The shared local/GitHub builder strips an optional leading `v` from the requested version and injects it through `internal/buildinfo.Version`; `hscope --version`, Runtime and CLI/browser ZIP manifests share that value. Unstamped source builds identify as `0.2.0-dev`.

## Public site gate

The public website is a static, bilingual GitHub Pages artifact built from `web/site/`. Home and Docs retain readable English fallbacks without JavaScript; Explore is explicitly synthetic and performs only same-origin GET requests against the committed fictional dataset. The site has no scanner endpoint, analytics, cookies, telemetry or third-party runtime assets. Concept photographs under `web/site/assets/generated/` are decorative and are recorded as not being product screenshots; product evidence uses committed Playwright synthetic captures.

`npm --prefix web run build:site` writes only ignored `dist/site/`. `npm --prefix web run test:site` checks translation, keyboard access, responsive layouts, subpath-safe assets, request boundaries, schema privacy and failure fallbacks. `npm --prefix web run capture:site` is an explicit maintenance action that refreshes the three reviewed README captures; ordinary builds never rewrite them. `node scripts/check-site-assets.mjs` validates PNG signatures, size bounds and bilingual README links. Ordinary CI runs all three checks. The Pages workflow accepts only `main`, including manual runs, and deploys only `dist/site/` after these checks pass. It rebuilds that output after tests to remove temporary shell-test fixtures before upload. It does not create a GitHub Release or modify repository contents. A passing local gate does not prove Pages is enabled or publicly deployed; verify the existing Pages source/custom domain before integration and verify the returned deployment URL afterwards.

## Reproduce the local gates

Requirements: Go 1.24+, Node 22+, npm, a supported Playwright browser, POSIX shell, curl, unzip and shasum. Synthetic demo/smoke refuse machines with managed agent configuration. The sample macOS caches below keep Go/browser downloads out of the repository; choose appropriate writable paths on Linux.

```sh
export GOCACHE=/private/tmp/harnessscope-go-cache
export GOMODCACHE=/private/tmp/harnessscope-go-mod
export PLAYWRIGHT_BROWSERS_PATH=/private/tmp/harnessscope-playwright
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
npm --prefix web ci
npm --prefix web run build
npm --prefix web run build:site
(cd web && npx tsc --noEmit)
git diff --exit-code -- internal/report/assets internal/server/assets
go build -trimpath -o bin/hscope ./cmd/hscope
npm --prefix web run test:visual
npm --prefix web run test:dashboard
npm --prefix web run test:site
node scripts/check-site-assets.mjs
node scripts/check-repository.mjs
for script in scripts/*.sh demo/*.sh; do sh -n "$script"; done
./scripts/smoke-release.sh ./bin/hscope
./scripts/check-third-party-notices.sh
node scripts/check-notices-test.mjs
node --test scripts/release-gates.test.mjs
./demo/run.sh
node scripts/check-demo-lifecycle.mjs
git diff --check
output=$(mktemp -d)
./scripts/build-release.sh v0.2.0 "$output"
(cd "$output" && shasum -a 256 -c SHA256SUMS)
unpack=$(mktemp -d)
tar -C "$unpack" -xzf "$output/harnessscope_v0.2.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
./scripts/check-release-archive.sh "$output/harnessscope_v0.2.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
./scripts/smoke-release.sh "$unpack/hscope" v0.2.0
```

Install a browser with `(cd web && npx playwright install chromium)` if necessary. If cached Chromium is absent but Chrome is installed, `PLAYWRIGHT_CHANNEL=chrome` is supported by both suites; record the actual platform/browser, never call that a Linux baseline. Linux CI installs its locked Chromium. A skipped visual test is not a passing comparison. See [baseline procedure](visual-baselines.md).

## Acceptance evidence map

The approved design's twelve criteria are mapped below (its acceptance section is Section 12; Section 10 is compatibility). Paths identify executable evidence, not a claim that every platform has run locally.

| Criterion | Direct evidence | Boundary |
|---|---|---|
| 1. Ready `serve --open`, no external requests | `internal/cli/serve_test.go` injected opener/startup tests; `internal/server/http_test.go`; dashboard authenticated navigation/request test; demo lifecycle | Opener tested by injection; browser visits synthetic ready service |
| 2. Seven usable views | `dashboard.spec.ts`, `fixes.spec.ts`, `drift-export.spec.ts`; inspected illustrative overview | Automated navigation, responsive geometry and screenshots; human product acceptance remains separate |
| 3. Graph interaction and origins | `dashboard.spec.ts` deterministic layout, pan/zoom, filter, keyboard and inspector cases | Synthetic committed browser graph; pan/zoom persistence across filter rerenders deferred |
| 4. Authentication/mutation rejection | `internal/server/security_test.go`, `routes_test.go`, `service_test.go`; release smoke all API route names | Missing token, origin/host, stale revision, non-SAFE and concurrent source checks |
| 5. SAFE browser transaction and rollback | `service-fixes.spec.ts` real Go service/filesystem round trip; `fixes.spec.ts` confirmation focus and stale behavior | Disposable fixture only; no real configuration |
| 6. Advanced analyzer positives/negatives/evidence | `internal/analyzers/advanced_test.go`; `demo/run.sh` all six advanced IDs | Structural rules and weakest-evidence propagation |
| 7. Deterministic added/removed/changed drift | `internal/snapshots/diff_test.go`; CLI snapshot tests; demo mutation and real-service browser save/compare | No semantic/raw-value diff |
| 8. Exact ZIP members, hashes, privacy | `internal/export/bundle_test.go`; CLI export tests; demo extraction; browser real-service download/hash assertions | Human review still required before sharing |
| 9. v0.1 compatibility | Full `go test ./...` including CLI/report/fixes and adapter fixtures; `web/tests/report.spec.ts` offline and no-JS | Schema v1/old backup paths retained; PREVIEW precedence not promoted |
| 10. Accessible status/reduced motion | `dashboard.spec.ts`, `fixes.spec.ts`, `drift-export.spec.ts`; 768px captures | Text severity, keyboard/focus, reduced motion and no-overflow assertions |
| 11. Four release builds/checksums | `scripts/build-release.sh`, CI cross-builds and release matrix; fresh native archive smoke | Foreign architectures cross-built, not executed locally |
| 12. Synthetic docs/demo provenance | `demo/conflicted-workspace/EXPECTED.md`; screenshot capture record; Task 13 gate report | Public image illustrative; reviewed Linux visual baseline still OPEN |

All output privacy assertions concern report/state/snapshot/bundle content. Local output-location messages intentionally include destination paths. Known root/home paths are collapsed; unknown external paths, arbitrary spellings or proprietary identifiers still require review.
