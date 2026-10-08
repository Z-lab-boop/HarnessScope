import { test, expect } from "@playwright/test";
import { spawn, execFile, type ChildProcessWithoutNullStreams } from "node:child_process";
import { mkdtemp, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

let root: string, launch: string, service: ChildProcessWithoutNullStreams;
test.beforeAll(async () => {
  root = await mkdtemp(join(tmpdir(), "hscope-fix-browser-"));
  await writeFile(join(root, "AGENTS.md"), "Original instruction\n");
  const binary = join(root, "fixture-server");
  await promisify(execFile)("go", ["build", "-o", binary, "./web/dashboard/tests/service-fixture"], { cwd: fileURLToPath(new URL("../../../", import.meta.url)), env: { ...process.env, GOCACHE: join(tmpdir(), "harnessscope-go-cache") } });
  service = spawn(binary, [root], { stdio: "pipe" });
  launch = await new Promise<string>((resolve, reject) => {
    let output = "";
    const timer = setTimeout(() => reject(new Error("Fixture startup timed out")), 10000);
    service.stdout.on("data", (chunk) => { output += chunk; if (output.includes("\n")) { clearTimeout(timer); resolve(output.trim()); } });
    service.once("error", (error) => { clearTimeout(timer); reject(error); });
    service.once("exit", () => { clearTimeout(timer); reject(new Error("Fixture exited during startup")); });
  });
});
test.afterAll(async () => {
  if (service && service.exitCode === null) {
    const stopped = new Promise<void>((resolve) => service.once("exit", () => resolve()));
    service.stdin.end(); await stopped;
  }
  if (root) await rm(root, { recursive: true, force: true });
});

test("refresh publishes changed plans before real service apply", async ({ page }) => {
  const url = new URL(launch); url.searchParams.set("view", "fixes");
  await page.goto(url.href);
  await expect(page.getByText("No fix plans in this snapshot.")).toBeVisible();
  await writeFile(join(root, "AGENTS.md"), "New private instruction\nNew private instruction\n");
  await page.getByRole("button", { name: "Refresh dry plan" }).click();
  await expect(page.getByRole("checkbox", { name: /^Select FIX-DUP-/ })).toBeEnabled();
  await page.getByRole("checkbox", { name: /^Select FIX-DUP-/ }).check();
  await page.getByRole("button", { name: "Apply selected SAFE fixes" }).click();
  await page.getByRole("button", { name: "Confirm apply" }).click();
  await expect(page.locator(".fix-feedback")).toContainText("Applied selected SAFE fixes. Scan revision 3 loaded.");
  await expect(page.getByRole("alert")).toBeHidden();
  await expect(page.getByRole("button", { name: /^Rollback / })).toBeVisible();
  expect(await readFile(join(root, "AGENTS.md"), "utf8")).toBe("New private instruction\n");
  await expect(page.locator("body")).not.toContainText("New private instruction");
});
