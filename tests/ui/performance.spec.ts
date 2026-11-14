import { expect, test, type Page } from "@playwright/test";
import { appendFile, mkdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { installRuntime } from "./runtime";

// The bridge is immediate: these measurements isolate React/DOM work from Go.
// Report cases use real HTML emitted by TestPerformanceStress.
test.skip(process.env.SPORTSMEET_PERF !== "1", "按需运行大数据页面压测");
test.describe.configure({ timeout: 180_000 });

async function settled(page: Page) {
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
}

async function record(data: Record<string, unknown>) {
  const line = JSON.stringify(data);
  console.log(line);
  const dir = process.env.SPORTSMEET_PERF_OUTPUT;
  if (dir) {
    await mkdir(dir, { recursive: true });
    await appendFile(join(dir, "browser.jsonl"), line + "\n");
  }
}

for (const width of [1280, 800]) {
  for (const count of [1000, 3000, 10000, 50000]) {
    test(`名单 ${count} 人 ${width}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await installRuntime(page, { athleteCount: count });
      const errors: string[] = [];
      page.on("pageerror", (err) => errors.push(err.message));
      await page.goto("/");
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "项目", exact: true }).click();
      await expect(page.getByRole("heading", { name: "项目", exact: true })).toBeVisible();
      await settled(page);
      await page.evaluate(() => performance.mark("perf-start"));
      await page.getByRole("button", { name: "报名", exact: true }).click();
      await page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^全部 \d/ }).click();
      await expect(page.getByText(`${count} 人`, { exact: true })).toBeVisible();
      await settled(page);
      const openMS = await page.evaluate(() => performance.now() - performance.getEntriesByName("perf-start").at(-1)!.startTime);
      const table = page.getByRole("table");
      const renderedRows = await table.getByRole("row").count();
      expect(renderedRows).toBeLessThan(100);
      await page.evaluate(() => performance.mark("perf-scroll"));
      await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
      await settled(page);
      const reachedEnd = await page.getByText(`运动员${count}`, { exact: true }).isVisible();
      const scrollState = await table.evaluate((el) => ({ scroll_top: el.scrollTop, scroll_height: el.scrollHeight, client_height: el.clientHeight }));
      const scrollMS = await page.evaluate(() => performance.now() - performance.getEntriesByName("perf-scroll").at(-1)!.startTime);
      const search = page.getByPlaceholder("搜索姓名、号码、学号");
      await page.evaluate(() => performance.mark("perf-search"));
      await search.fill(`运动员${count}`);
      await expect(table.getByRole("row")).toHaveCount(2);
      await settled(page);
      const searchMS = await page.evaluate(() => performance.now() - performance.getEntriesByName("perf-search").at(-1)!.startTime);
      expect(errors).toEqual([]);
      await record({ operation: "AthletesUI", count, width, open_ms: openMS, scroll_ms: scrollMS, search_ms: searchMS, rendered_rows: renderedRows, reached_end: reachedEnd, ...scrollState });
      expect(reachedEnd, "虚拟列表应能滚动到最后一名运动员").toBe(true);
    });
  }
}

test("虚拟列表占位高度诊断", async ({ page }) => {
  await installRuntime(page, { athleteCount: 1000 });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "项目", exact: true }).click();
  await page.getByRole("button", { name: "报名", exact: true }).click();
  await page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^全部 \d/ }).click();
  const table = page.getByRole("table");
  await settled(page);
  const before = await table.evaluate((el) => el.scrollHeight);
  expect(before).toBeGreaterThan(39_000);
  await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
  await expect(page.getByText("运动员1000", { exact: true })).toBeVisible();
  const after = await table.evaluate((el) => el.scrollHeight);
  await record({ operation: "VirtualSpacerDiagnostic", count: 1000, before_scroll_height: before, final_scroll_height: after, reached_end: true });
});

for (const scale of ["school", "large", "extreme", "wide"]) {
  test(`真实秩序册分页 ${scale}`, async ({ page }) => {
    const dir = process.env.SPORTSMEET_PERF_OUTPUT;
    test.skip(!dir, "需要 Go 压测生成的秩序册 HTML");
    const html = await readFile(join(dir!, `${scale}-startlist.html`), "utf8");
    await installRuntime(page);
    await page.route("**/__perf-report", (route) => route.fulfill({ contentType: "text/html", body: html }));
    await page.addInitScript(() => {
      let html = "";
      (globalThis as any).__perfSource = null;
      (globalThis as any).__perfLongTasks = [];
      (globalThis as any).__loadPerfReport = async () => {
        html = await (await fetch("/__perf-report")).text();
        const doc = new DOMParser().parseFromString(html, "text/html");
        (globalThis as any).__perfSource = { rows: doc.querySelectorAll("tbody > tr").length, heats: doc.querySelectorAll(".heat").length };
      };
      new PerformanceObserver((list) => {
        for (const task of list.getEntries()) (globalThis as any).__perfLongTasks.push(task.duration);
      }).observe({ entryTypes: ["longtask"] });
      const bridge = (globalThis as any).mygo;
      const call = bridge.call;
      bridge.call = (method: string, ...args: unknown[]) => method === "Reports.Render" ? Promise.resolve(html) : call(method, ...args);
    });
    await page.setViewportSize({ width: 1280, height: 800 });
    const errors: string[] = [];
    page.on("pageerror", (err) => errors.push(err.message));
    await page.goto("/");
    await page.evaluate(() => (globalThis as any).__loadPerfReport());
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "项目", exact: true }).click();
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Performance.enable");
    const before = (await cdp.send("Performance.getMetrics")).metrics;
    await page.evaluate(() => { (globalThis as any).__perfLongTasks = []; performance.mark("perf-report"); });
    await page.getByRole("button", { name: "打印", exact: true }).click();
    await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/, { timeout: 150_000 });
    await settled(page);
    const totalMS = await page.evaluate(() => performance.now() - performance.getEntriesByName("perf-report").at(-1)!.startTime);
    const after = (await cdp.send("Performance.getMetrics")).metrics;
    const delta = (name: string) => ((after.find((m) => m.name === name)?.value ?? 0) - (before.find((m) => m.name === name)?.value ?? 0)) * 1000;
    const pages = await page.frameLocator('iframe[title="预览"]').locator(".pv-page").count();
    expect(pages).toBeGreaterThan(0);
    expect(errors).toEqual([]);
    const source = await page.evaluate(() => (globalThis as any).__perfSource);
    const layout = await page.frameLocator('iframe[title="预览"]').locator("body").evaluate((body) => ({
      rows: body.querySelectorAll("tbody > tr").length, heats: body.querySelectorAll(".heat").length,
      overflow_pages: [...body.querySelectorAll<HTMLElement>(".pv-body")].filter((el) => el.scrollHeight > el.clientHeight + 1).length,
      empty_pages: [...body.querySelectorAll(".pv-body")].filter((el) => !el.textContent?.trim()).length,
    }));
    expect(layout.rows).toBe(source.rows);
    expect(layout.heats).toBe(source.heats);
    expect(layout.overflow_pages).toBe(0);
    expect(layout.empty_pages).toBe(0);
    const longestTask = await page.evaluate(() => Math.max(0, ...(globalThis as any).__perfLongTasks));
    await page.screenshot({ path: join(dir!, `${scale}-preview.png`) });
    await record({ operation: "ReportPagination", scale, html_bytes: Buffer.byteLength(html), pages, total_ms: totalMS,
      longest_task_ms: longestTask, ...layout, layout_ms: delta("LayoutDuration"), style_ms: delta("RecalcStyleDuration"), task_ms: delta("TaskDuration") });
  });
}
