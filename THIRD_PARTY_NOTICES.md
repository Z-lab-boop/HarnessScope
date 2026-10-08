# Third-party notices

Dependency snapshot: `go.sum` SHA-256 `3e28500a14d8ff573971c57052bd4123be34c4f0342cb10dff0633c6c141ebb1`; `web/package-lock.json` SHA-256 `f48faf12842477295d3051b87b2e752550c1fada97396ce3abf0dea15847cfc9`.

## Go runtime dependencies

| Module | Version | License | Upstream |
|---|---:|---|---|
| github.com/spf13/cobra | 1.10.1 | Apache-2.0 | https://github.com/spf13/cobra |
| github.com/spf13/pflag | 1.0.9 | BSD-3-Clause | https://github.com/spf13/pflag |
| github.com/inconshreveable/mousetrap | 1.1.0 | Apache-2.0 | https://github.com/inconshreveable/mousetrap |
| github.com/pelletier/go-toml/v2 | 2.2.4 | MIT | https://github.com/pelletier/go-toml |
| gopkg.in/yaml.v3 | 3.0.1 | MIT and Apache-2.0 | https://github.com/go-yaml/yaml |

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
