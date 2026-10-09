import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { startCaptureServer } from "./capture-server.mjs";

test("capture owns its listener and rejects a foreign HTTP 200 on a busy port", async () => {
  const root = await mkdtemp(join(tmpdir(), "hscope-capture-server-"));
  const foreign = createServer((request, response) => response.end("foreign server"));
  let child;
  try {
    await writeFile(join(root, "index.html"), "owned capture fixture");
    foreign.listen(0, "127.0.0.1");
    await once(foreign, "listening");
    const port = foreign.address().port;
    const foreignResponse = await fetch(`http://127.0.0.1:${port}`);
    assert.equal(foreignResponse.status, 200);
    assert.equal(await foreignResponse.text(), "foreign server");

    // A successful HTTP response cannot stand in for the child's bind success.
    await assert.rejects(startCaptureServer({ root, port }), /exited before listening.*EADDRINUSE/s);
    const owned = await startCaptureServer({ root });
    child = owned.server;
    assert.notEqual(owned.base, `http://127.0.0.1:${port}`);
    assert.equal(await (await fetch(owned.base)).text(), "owned capture fixture");
    await assert.rejects(startCaptureServer({ root: join(root, "missing") }), /exited before listening.*ENOENT/s);
  } finally {
    if (child && child.exitCode === null) {
      const exited = once(child, "exit");
      child.kill("SIGTERM");
      await exited;
    }
    foreign.closeAllConnections();
    await new Promise(resolve => foreign.close(resolve));
    await rm(root, { recursive: true, force: true });
  }
});
