# Visual evidence and open Linux gate

The public `assets/dashboard-overview.png` is an illustrative local macOS Chromium capture of the committed synthetic `web/dashboard/tests/fixture-state.json`, rendered by `dashboard.spec.ts`. It is not a Linux regression baseline or evidence from real user configuration.

Capture record: 2026-10-09, macOS arm64, Chromium 141.0.7390.37 (Playwright 1.56.1 cached build 1194), 1440 × 1000 viewport with a 1440 × 1060 full-page image and reduced motion. Fixture commit: `44390063766fb53562ab77a2b06696d5b177d2e8`; fixture SHA-256: `efc2c808745a1007332fa63e1d2b19075e6cd89f53d32c368918c0a359a5bebf`. Image SHA-256: `72a0ca530a7e805f749a4a3c64ee50bc665b0fe24d8e26441a1077387b040878`. The implementing agent visually inspected the actual pixels at full size: all cards, tier labels, navigation, inspector and evidence footer are readable and unclipped. This records agent visual inspection, not an independent human review.

No reviewed Linux report or dashboard baseline is currently checked in. The old `web/tests/__snapshots__/report.png` has unverified platform provenance and is not used as a regression baseline. Functional Playwright tests run on both CI platforms; the two explicitly named Linux visual tests are skipped until reviewed artifacts exist. This is an **OPEN release gate**, not a passing visual comparison.

To close the gate on a Linux host matching CI, install the locked Playwright Chromium, run both suites with `HARNESSSCOPE_REVIEWED_LINUX_BASELINE=1` and `--update-snapshots`, and inspect the generated `report-linux.png` and `dashboard-linux.png`. Record Linux distribution, browser version, viewport, fixture commit, reviewer and image hashes here. Commit both inspected images, enable the environment variable in CI, and run without `--update-snapshots`. Only Linux may compare these images. Never copy the illustrative macOS image into either baseline path.

CI's `reviewed-linux-visual-gate` deliberately fails if the two baseline files are absent. Release publication depends on this gate. Linux execution and the review itself cannot be substituted by a cross-compiled binary or local macOS browser run.
