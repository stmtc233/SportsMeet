import { expect, test } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expectReport, installRuntime, selectReport } from "./runtime";

let html: string;
test.beforeAll(() => {
  const dir = mkdtempSync(join(tmpdir(), "sportsmeet-entries-"));
  try {
    const path = join(dir, "entries.html");
    execFileSync("go", ["test", "./internal/service", "-run", "^TestEntriesReportLongTable$", "-count=1"], {
      env: { ...process.env, SPORTSMEET_ENTRIES_REPORT_OUTPUT: path },
    });
    html = readFileSync(path, "utf8");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

for (const width of [1280, 800]) for (const colorScheme of ["light", "dark"] as const) {
  test(`报名表预览、分页与打印 ${width} ${colorScheme}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 800 });
    await page.emulateMedia({ colorScheme });
    await installRuntime(page);
    await page.addInitScript((html) => {
      const bridge = (globalThis as any).mygo;
      const call = bridge.call;
      (globalThis as any).__entryReportCalls = [];
      bridge.call = (method: string, ...args: unknown[]) => {
        if (method === "Reports.Targets") return Promise.resolve([{ id: "event", name: "男子100m" }, { id: "jump", name: "女子跳远" }]);
        if (method === "Reports.Render" && args[0] === "entries") {
          (globalThis as any).__entryReportCalls.push(args);
          return Promise.resolve(html);
        }
        return call(method, ...args);
      };
    }, html);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "项目", exact: true }).click();
    await page.getByRole("button", { name: "打印", exact: true }).click();
    await selectReport(page, "报名表");
    await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);
    const body = page.frameLocator('iframe[title="预览"]').locator("body");
    await expect(body.locator("tbody tr")).toHaveCount(140);
    const layout = await body.evaluate((body) => {
      const pages = [...body.querySelectorAll<HTMLElement>(".pv-body")];
      return {
        pages: pages.length,
        overflow: pages.some((p) => p.scrollHeight > p.clientHeight + 1 || p.scrollWidth > p.clientWidth + 1),
        empty: pages.some((p) => !p.textContent?.trim()),
        headers: pages.filter((p) => p.querySelector("tbody tr")).every((p) => p.querySelector("thead th")?.textContent === "序号"),
        last: pages.at(-1)?.querySelector("h2")?.textContent,
      };
    });
    expect(layout.pages).toBeGreaterThan(2);
    expect(layout.overflow).toBe(false);
    expect(layout.empty).toBe(false);
    expect(layout.headers).toBe(true);
    expect(layout.last).toContain("女子跳远");
    await expect(body.getByText("续下页", { exact: true }).first()).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByRole("listbox", { includeHidden: true }).waitFor({ state: "detached" });
    await page.screenshot({ path: testInfo.outputPath("entries.png") });

    const print = page.getByRole("button", { name: "打印", exact: true }).last();
    await expect(print).toBeEnabled();
    await print.click();
    const printed = await page.evaluate(() => (globalThis as any).__prints.at(-1) as string);
    expect(printed).toContain('data-title="报名表"');
    expect(printed).toContain("选手140");
    expect(printed).toContain("共 " + layout.pages + " 页");
    await page.getByRole("button", { name: "取消全选", exact: true }).click();
    await expect(page.getByText("未选择项目", { exact: true })).toBeVisible();
    await expect(print).toBeDisabled();
    await page.getByRole("checkbox", { name: "男子100m", exact: true }).check();
    await expect.poll(() => page.evaluate(() => (globalThis as any).__entryReportCalls.at(-1))).toEqual(["entries", "meet", ["event"]]);
    await page.getByRole("button", { name: "项目", exact: true }).click();
    await page.getByRole("button", { name: "打印", exact: true }).click();
    await expectReport(page, "报名表");
  });
}
