import { build } from "esbuild";
import { cp, mkdir, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const out = new URL("../../dist/site/", import.meta.url);
await rm(out, { recursive: true, force: true });
await mkdir(new URL("assets/", out), { recursive: true });
for (const page of ["index.html", "explore.html", "docs.html"]) {
  await cp(new URL(page, import.meta.url), new URL(page, out));
}
await build({ entryPoints: [fileURLToPath(new URL("src/site.ts", import.meta.url))], bundle: true, minify: true, format: "esm", define: { "process.env.NODE_ENV": '"production"' }, outfile: fileURLToPath(new URL("assets/site.js", out)) });
await build({ entryPoints: [fileURLToPath(new URL("src/site.css", import.meta.url))], bundle: true, minify: true, outfile: fileURLToPath(new URL("assets/site.css", out)) });
for (const dir of ["assets", "data"]) {
  await cp(new URL(`${dir}/`, import.meta.url), new URL(`${dir}/`, out), { recursive: true }).catch(error => {
    if (error.code !== "ENOENT") throw error;
  });
}
