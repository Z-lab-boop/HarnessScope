import { test, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";

test.use({ locale: "zh-CN" });

for (const path of ["/", "/explore.html", "/docs.html"]) {
  test(`${path} has an English-first accessible shell`, async ({ page }) => {
    const requests: string[] = [];
    page.on("request", request => requests.push(request.url()));
    await page.goto(path);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
    await expect(page.getByRole("button", { name: "切换到中文" })).toBeVisible();
    expect(requests.every(url => new URL(url).origin === new URL(page.url()).origin)).toBe(true);
  });
}

test("language selection is explicit and persists without cookies", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "切换到中文" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("navigation", { name: "主导航" })).toBeVisible();
  expect(await page.context().cookies()).toEqual([]);
  await page.goto("/docs.html");
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("link", { name: "文档与下载", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Switch to English" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
});

test("English-first ignores browser locale and invalid stored values", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("harnessscope.locale", "fr"));
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  expect(await page.evaluate(() => localStorage.getItem("harnessscope.locale"))).toBe("fr");
});

test("repository subpath serves CSS, JavaScript, data and images", async ({ page }) => {
  // Temporary build-output fixtures exercise future asset mounts without
  // requiring Task 2's production images or Task 4's dataset.
  const out = new URL("../../../dist/site/", import.meta.url);
  await mkdir(new URL("data/", out), { recursive: true });
  await writeFile(new URL("data/shell-test.json", out), '{"synthetic":true}');
  await writeFile(new URL("assets/shell-test.svg", out), '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>');
  const responses: { url: string; status: number }[] = [];
  page.on("response", response => responses.push({ url: response.url(), status: response.status() }));
  await page.goto("/HarnessScope/explore.html");
  await expect(page.getByRole("button", { name: "切换到中文" })).toBeVisible();
  const assets = await page.evaluate(async () => {
    const urls = ["./assets/site.css", "./assets/site.js", "./data/shell-test.json", "./assets/shell-test.svg"];
    return Promise.all(urls.map(async url => {
      const response = await fetch(url);
      return { url: response.url, status: response.status, type: response.headers.get("content-type") };
    }));
  });
  expect(assets.map(asset => asset.status)).toEqual([200, 200, 200, 200]);
  expect(assets.map(asset => asset.type)).toEqual(["text/css; charset=utf-8", "text/javascript; charset=utf-8", "application/json; charset=utf-8", "image/svg+xml"]);
  expect(responses.every(response => response.status === 200 && new URL(response.url).pathname.startsWith("/HarnessScope/"))).toBe(true);
});

test("shell remains readable without JavaScript and fits a narrow viewport", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 360, height: 800 } });
  const page = await context.newPage();
  for (const path of ["/", "/explore.html", "/docs.html"]) {
    await page.goto(path);
    await expect(page.getByRole("main")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await context.close();
});

test("static server rejects encoded traversal", async ({ request }) => {
  const response = await request.get("/HarnessScope/%2e%2e%2fpackage.json");
  expect(response.status()).toBe(400);
});

function contrast(first: string, second: string) {
  const luminance = (color: string) => {
    const channels = color.match(/[\d.]+/g)!.slice(0, 3).map(value => {
      const channel = Number(value) / 255;
      return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4;
    });
    return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
  };
  const values = [luminance(first), luminance(second)].sort((a, b) => b - a);
  return (values[0] + .05) / (values[1] + .05);
}

for (const width of [360, 1440]) for (const path of ["/", "/explore.html", "/docs.html"]) {
  test(`keyboard focus contrasts with light and dark surfaces on ${path} at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(path);
    if (path === "/explore.html") {
      await expect(page.locator("[data-explore-workbench]")).toBeVisible();
      await page.getByRole("button", { name: "Show conflicts" }).click();
      await page.getByRole("button", { name: "Preview safe fix" }).click();
    }
    // Keyboard modality also makes programmatic focus match :focus-visible.
    await page.keyboard.press("Tab");
    const controls = page.locator('a[href], button, select, [tabindex]:not([tabindex="-1"])');
    let darkControls = 0;
    let lightControls = 0;
    for (const control of await controls.all()) {
      if (!await control.isVisible()) continue;
      await control.focus();
      await control.scrollIntoViewIfNeeded();
      await expect(control).toBeFocused();
      await expect(control).toBeInViewport();
      const ring = await control.evaluate(element => {
        const style = getComputedStyle(element);
        const graph = element.querySelector(".graph-hit");
        let background = "";
        for (let parent = element.parentElement; parent; parent = parent.parentElement) {
          const color = getComputedStyle(parent).backgroundColor;
          if (color !== "rgba(0, 0, 0, 0)" && color !== "transparent") { background = color; break; }
        }
        return {
          visible: element.matches(":focus-visible"), background,
          color: graph ? getComputedStyle(graph).stroke : style.outlineColor,
          width: graph ? getComputedStyle(graph).strokeWidth : style.outlineWidth,
          style: graph ? "solid" : style.outlineStyle,
          offset: graph ? "4px" : style.outlineOffset,
        };
      });
      expect(ring.visible).toBe(true);
      expect(ring.style).toBe("solid");
      expect(parseFloat(ring.width)).toBeGreaterThanOrEqual(3);
      expect(parseFloat(ring.offset)).toBeGreaterThanOrEqual(2);
      expect(contrast(ring.color, ring.background), `${path}: ${await control.textContent()}`).toBeGreaterThanOrEqual(3);
      if (ring.background === "rgb(6, 21, 40)") {
        darkControls++;
        expect(ring.color).toBe("rgb(255, 102, 127)");
      } else {
        lightControls++;
        // The default must also remain visible on the inspector's light surface.
        expect(contrast(ring.color, "rgb(238, 232, 218)")).toBeGreaterThanOrEqual(3);
        expect(contrast(ring.color, "rgb(231, 223, 207)")).toBeGreaterThanOrEqual(3);
      }
    }
    expect(lightControls).toBeGreaterThan(0);
    if (path !== "/docs.html") expect(darkControls).toBeGreaterThan(0);
  });
}
