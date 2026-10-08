# Third-party notices

Dependency snapshot re-audited for v0.2; versions are unchanged. The Go checksum file additionally records the two existing documentation-tool module ZIP hashes verified during the audit. Exact anchored records checked by `scripts/check-third-party-notices.sh`:

go.sum SHA-256: 0b31a622275fd8f9f7ac00ea95aa344584bb7405f9ea4704f133cee12353a3c8
web/package-lock.json SHA-256: f48faf12842477295d3051b87b2e752550c1fada97396ce3abf0dea15847cfc9

## Go runtime dependencies

| Module | Version | License | Upstream |
|---|---:|---|---|
| github.com/spf13/cobra | 1.10.1 | Apache-2.0 | https://github.com/spf13/cobra |
| github.com/spf13/pflag | 1.0.9 | BSD-3-Clause | https://github.com/spf13/pflag |
| github.com/inconshreveable/mousetrap | 1.1.0 | Apache-2.0 | https://github.com/inconshreveable/mousetrap |
| github.com/pelletier/go-toml/v2 | 2.2.4 | MIT | https://github.com/pelletier/go-toml |
| gopkg.in/yaml.v3 | 3.0.1 | MIT and Apache-2.0 | https://github.com/go-yaml/yaml |

Additional modules in the exact `go list -m all` graph (dependency tooling/tests; not imported by the release command):

| Module | Version | License | Upstream |
|---|---|---|---|
| github.com/cpuguy83/go-md2man/v2 | 2.0.6 | MIT | https://github.com/cpuguy83/go-md2man |
| github.com/russross/blackfriday/v2 | 2.1.0 | BSD-2-Clause | https://github.com/russross/blackfriday |
| gopkg.in/check.v1 | 0.0.0-20161208181325-20d25e280405 | BSD-2-Clause | https://github.com/go-check/check |

Required upstream notice for `gopkg.in/yaml.v3`:

> Copyright 2011-2016 Canonical Ltd. Licensed under the Apache License, Version 2.0. Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an AS IS BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

## Web build and test dependency graph

These packages are development-only; the release binary embeds the generated CSS and JavaScript rather than Node packages.

| Package family | Locked version | License | Upstream |
|---|---:|---|---|
| typescript | 5.9.3 | Apache-2.0 | https://github.com/microsoft/TypeScript |
| esbuild and all optional `@esbuild/*` platform packages | 0.25.11 | MIT | https://github.com/evanw/esbuild |
| @playwright/test, playwright, playwright-core | 1.56.1 | Apache-2.0 | https://github.com/microsoft/playwright |
| @types/node | 24.10.0 | MIT | https://github.com/DefinitelyTyped/DefinitelyTyped |
| undici-types | 7.16.0 | MIT | https://github.com/nodejs/undici |
| fsevents | 2.3.2 | MIT | https://github.com/fsevents/fsevents |

The full Apache-2.0 text is included in `LICENSE`. MIT and BSD dependencies retain their copyright and license files in their upstream distributions; their terms permit redistribution in this Apache-2.0 project.

The lock enumerates exactly 34 npm packages. The 26 optional esbuild packages are: aix-ppc64, android-arm, android-arm64, android-x64, darwin-arm64, darwin-x64, freebsd-arm64, freebsd-x64, linux-arm, linux-arm64, linux-ia32, linux-loong64, linux-mips64el, linux-ppc64, linux-riscv64, linux-s390x, linux-x64, netbsd-arm64, netbsd-x64, openbsd-arm64, openbsd-x64, openharmony-arm64, sunos-x64, win32-arm64, win32-ia32, win32-x64 (all under `@esbuild/`, version 0.25.11, MIT). The other eight packages are individually covered above. The graph has no added runtime frontend dependency. Full runtime dependency license texts accompany archives under `docs/licenses/`.
