import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { installRuntime, selectReport } from "./runtime";

const style = readFileSync("internal/report/style.css", "utf8");
const rows = (n: number, prefix: string) => Array.from({ length: n }, (_, i) => `<tr data-line="${prefix}${i}"><td>${prefix}${i}</td><td>运动员</td></tr>`).join("");
const table = (n: number, prefix: string) => `<table data-table="${prefix}"><thead><tr><th>号码</th><th>姓名</th></tr></thead><tbody>${rows(n, prefix)}</tbody></table>`;
const document = (body: string) => `<!doctype html><html><head><style>${style}</style></head><body>${body}</body></html>`;

async function installReport(page: Page, html: string) {
  await installRuntime(page);
  await page.addInitScript(({ html, simple }) => {
    const bridge = (globalThis as any).mygo;
    const call = bridge.call;
    bridge.call = (method: string, ...args: unknown[]) => method === "Reports.Render" ? Promise.resolve(args[0] === "results" ? simple : html) : call(method, ...args);
  }, { html, simple: document("<section><h2>成绩册切换完成</h2><p>结果</p></section>") });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "项目", exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
}

test("分页保留整组、重复表头、强制换页与全部记录", async ({ page }) => {
  const grid = Array.from({ length: 8 }, (_, i) => `<div class="heat"><h4>第${i + 1}组</h4>${table(12, `h${i}-`)}</div>`).join("");
  const html = document(`<header><h1>分页测试</h1></header>
    <section class="event" style="break-before:page"><h2>长表</h2>${table(130, "a")}</section>
    <section class="event" style="break-before:page"><h2>分组</h2><div class="heats">${grid}</div></section>
    <section class="sheet break"><h2>检录单独分页</h2>${table(4, "z")}</section>`);
  await installReport(page, html);
  await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);
  const body = page.frameLocator('iframe[title="预览"]').locator("body");
  const layout = await body.evaluate((body) => {
    const pages = [...body.querySelectorAll<HTMLElement>(".pv-body")];
    const tablePages = pages.filter((p) => p.querySelector('table[data-table="a"] tbody tr'));
    return {
      lines: [...body.querySelectorAll("tbody > tr[data-line]")].map((r) => r.getAttribute("data-line")),
      overflow: pages.filter((p) => p.scrollHeight > p.clientHeight + 1).length,
      empty: pages.filter((p) => !p.textContent?.trim()).length,
      repeatedHeaders: tablePages.every((p) => !!p.querySelector('table[data-table="a"] thead')),
      tablePages: tablePages.length,
      heats: [...body.querySelectorAll(".heat")].map((h) => h.querySelectorAll("tbody tr").length),
      sheetStartsPage: !!body.querySelector(".pv-body > .sheet.break"),
    };
  });
  expect(layout.lines).toHaveLength(130 + 8 * 12 + 4);
  expect(new Set(layout.lines).size).toBe(layout.lines.length);
  expect(layout.overflow).toBe(0);
  expect(layout.empty).toBe(0);
  expect(layout.repeatedHeaders).toBe(true);
  expect(layout.tablePages).toBeGreaterThan(1);
  expect(layout.heats).toEqual(Array(8).fill(12));
  expect(layout.sheetStartsPage).toBe(true);
});

test("分页过程中切换报表并离开页面", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  await installReport(page, document(`<section><h2>长报表</h2>${table(1500, "a")}</section>`));
  await selectReport(page, "成绩册");
  await expect(page.frameLocator('iframe[title="预览"]').getByText("成绩册切换完成", { exact: true })).toBeVisible();
  await expect(page.locator(".subbar__count")).toHaveText("1 页");
  await selectReport(page, "秩序册");
  await page.getByRole("button", { name: "运动员", exact: true }).click();
  await expect(page.getByRole("heading", { name: "运动员", exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});
