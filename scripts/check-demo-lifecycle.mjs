import { spawn } from "node:child_process";
import assert from "node:assert/strict";

const child = spawn("./demo/dashboard.sh", { stdio: ["ignore", "pipe", "pipe"] });
let stdout = "", stderr = "";
child.stdout.on("data", (chunk) => { stdout += chunk; });
child.stderr.on("data", (chunk) => { stderr += chunk; });
const exited = new Promise((resolve, reject) => {
  child.once("error", reject);
  child.once("exit", (code, signal) => resolve({ code, signal }));
});
const timer = setTimeout(() => { child.kill("SIGTERM"); }, 60000);
try {
  await Promise.race([
    new Promise((resolve) => {
      const observe = () => { if (stdout.includes("\n")) resolve(); else setTimeout(observe, 50).unref(); };
      observe();
    }),
    exited.then(() => { throw new Error("Demo exited before startup (launch output withheld)"); }),
  ]);
  const lines = stdout.trim().split("\n");
  assert.equal(lines.length, 1, "exactly one launch URL");
  const url = new URL(lines[0]);
  assert.equal(url.hostname, "127.0.0.1");
  assert.ok(url.hash.startsWith("#token="));
  assert.equal((await fetch(url.origin)).status, 200);
  child.kill("SIGINT");
  assert.deepEqual(await exited, { code: 0, signal: null });
  await assert.rejects(fetch(url.origin));
  assert.equal(stdout.trim().split("\n").length, 1);
  assert.equal(stderr, "");
  console.log("Synthetic dashboard port 0, one launch URL, Ctrl-C and closed listener PASS");
} finally {
  clearTimeout(timer);
  if (child.exitCode === null) child.kill("SIGTERM");
}
