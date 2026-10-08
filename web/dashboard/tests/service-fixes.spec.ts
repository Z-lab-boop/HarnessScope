import { test, expect } from "@playwright/test";
import { spawn, execFile, type ChildProcessWithoutNullStreams } from "node:child_process";
import { mkdtemp, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { createHash } from "node:crypto";

let root: string, launch: string, service: ChildProcessWithoutNullStreams;
test.beforeAll(async () => {
  root = await mkdtemp(join(tmpdir(), "hscope-fix-browser-"));
  await writeFile(join(root, "AGENTS.md"), "Original instruction\n");
  const binary = join(root, "fixture-server");
  await promisify(execFile)("go", ["build", "-o", binary, "./web/dashboard/tests/service-fixture"], { cwd: fileURLToPath(new URL("../../../", import.meta.url)), env: { ...process.env, GOCACHE: process.env.GOCACHE || join(tmpdir(), "harnessscope-go-cache") } });
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

test("real service browser round trip applies, rolls back, snapshots and exports", async ({ page }) => {
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
  const rollback = page.getByRole("button", { name: /^Rollback / });
  const backupID = (await rollback.textContent())!.replace(/^Rollback /, "");
  await rollback.click();
  await page.getByLabel("Type backup ID to confirm").fill(backupID);
  await page.getByRole("button", { name: "Confirm rollback" }).click();
  await expect(page.getByText("REV 4", { exact: true })).toBeVisible();
  expect(await readFile(join(root, "AGENTS.md"), "utf8")).toBe("New private instruction\nNew private instruction\n");
  await page.getByRole("link", { name: "Drift", exact: true }).click();
  await page.getByLabel("Snapshot name").fill("browser-base");
  await page.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByLabel("Baseline", { exact: true })).toHaveValue("browser-base");
  await page.getByRole("button", { name: "Compare baseline" }).click();
  await expect(page.getByText("No normalized drift from this baseline.")).toBeVisible();
  await page.getByRole("link", { name: "Export", exact: true }).click();
  const pending = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download diagnostic ZIP" }).click();
  const download = await pending;
  const archive = join(root, "browser-diagnostic.zip");
  await download.saveAs(archive);
  const extract = async (name: string) => (await promisify(execFile)("unzip", ["-p", archive, name], { encoding: "buffer" })).stdout;
  const names = (await promisify(execFile)("unzip", ["-Z1", archive])).stdout.trim().split("\n").sort();
  expect(names).toEqual(["README.txt", "drift.json", "manifest.json", "report.html", "report.json"]);
  const manifest = JSON.parse((await extract("manifest.json")).toString());
  for (const member of manifest.members) {
    const bytes = await extract(member.name);
    expect(createHash("sha256").update(bytes).digest("hex")).toBe(member.sha256);
    expect(bytes.toString()).not.toContain(root);
    expect(bytes.toString()).not.toContain("New private instruction");
  }
});
