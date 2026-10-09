import { createServer } from "node:http";
import { readFile, realpath } from "node:fs/promises";
import { extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

// Resolve the CLI path from this script, independently of Playwright's cwd.
const root = await realpath(fileURLToPath(new URL(`${process.argv[2] ?? "../../dist/site"}/`, import.meta.url)));
const types = { ".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".json": "application/json; charset=utf-8", ".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".ico": "image/x-icon" };
createServer(async (request, response) => {
  try {
    let path = decodeURIComponent((request.url ?? "/").split("?")[0]);
    if (path.includes("\\") || path.includes("\0") || path.split("/").includes("..")) {
      response.writeHead(400).end("Invalid path");
      return;
    }
    if (path === "/HarnessScope") {
      response.writeHead(301, { Location: "/HarnessScope/" }).end();
      return;
    }
    if (path.startsWith("/HarnessScope/")) path = path.slice("/HarnessScope".length);
    if (path.endsWith("/")) path += "index.html";
    const file = await realpath(resolve(root, `.${path}`));
    if (!file.startsWith(root + sep)) {
      response.writeHead(400).end("Invalid path");
      return;
    }
    const body = await readFile(file);
    response.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream", "Cache-Control": "no-store" });
    response.end(request.method === "HEAD" ? undefined : body);
  } catch (error) {
    response.writeHead(error instanceof URIError ? 400 : 404).end("Unavailable");
  }
}).listen(4177, "127.0.0.1");
