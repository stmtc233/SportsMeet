import { expect, test, type Page } from "@playwright/test";
import { reportStyle } from "./reportStyle";
import { installRuntime, selectReport } from "./runtime";

const style = reportStyle;
const rows = (n: number, prefix: string) => Array.from({ length: n }, (_, i) => `<tr data-line="${prefix}${i}"><td>${prefix}${i}</td><td>运动员</td></tr>`).join("");
const table = (n: number, prefix: string) => `<table data-table="${prefix}"><thead><tr><th>号码</th><th>姓名</th></tr></thead><tbody>${rows(n, prefix)}</tbody></table>`;
const document = (body: string) => `<!doctype html><html><head><style>${style}</style></head><body>${body}</body></html>`;

async function installReport(page: Page, html: string) {
  await installRuntime(page);
  await page.addInitScript(({ html, simple }) => {
    const bridge = (globalThis as any).mygo;
    const call = bridge.call;
    bridge.call = (method: string, ...args: unknown[]) => method === "Reports.Render" ? Promise.resolve({ html: args[0] === "results" ? simple : html, warnings: [] }) : call(method, ...args);
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
  await page.getByRole("button", { name: "报名", exact: true }).click();
  await expect(page.getByRole("heading", { name: "报名", exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("纸张、边距、页码与页眉随报表的参数", async ({ page }) => {
  const csp = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:">`;
  // A3 landscape with 20mm margins: Go's parameters block, after the default look.
  const params = ":root{--paper-w:420mm;--paper-h:297mm;--m-t:20mm;--m-r:10mm;--m-b:20mm;--m-l:10mm}@page{size:420mm 297mm;margin:20mm 10mm}";
  const html = `<!doctype html><html><head><meta charset="UTF-8">${csp}<style>${style}${params}</style></head>
    <body data-paper="420x297" data-margin="20,10,20,10" data-meet="校运会" data-title="秩序册" data-head-left="内部资料" data-pagenum="dash">
    <section class="event" style="break-before:page"><h2>长表</h2>${table(90, "a")}</section></body></html>`;
  await installReport(page, html);
  await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);
  const frame = page.frameLocator('iframe[title="预览"]');
  const sheet = await frame.locator("body").evaluate((body) => {
    const pages = [...body.querySelectorAll<HTMLElement>(".pv-page")];
    const px = (mm: number) => (mm * 96) / 25.4;
    return {
      count: pages.length,
      width: pages[0]!.offsetWidth / px(420),
      height: pages[0]!.offsetHeight / px(297),
      padding: getComputedStyle(pages[0]!).padding,
      feet: pages.map((p) => p.querySelector(".pv-foot")?.textContent),
      secondHead: [...(pages[1]?.querySelectorAll(".pv-head span") ?? [])].map((s) => s.textContent),
      firstHead: pages[0]!.querySelector(".pv-head") !== null,
      pageRule: [...body.ownerDocument.querySelectorAll("style")].some((s) => s.textContent?.includes("@page { size: 420mm 297mm; margin: 0; }")),
      csp: body.ownerDocument.querySelector('meta[http-equiv="Content-Security-Policy"]') !== null,
    };
  });
  expect(sheet.count).toBeGreaterThan(1);
  expect(sheet.width).toBeCloseTo(1, 2);
  expect(sheet.height).toBeCloseTo(1, 2);
  expect(sheet.feet[0]).toBe("- 1 -");
  expect(sheet.feet.at(-1)).toBe(`- ${sheet.count} -`);
  expect(sheet.firstHead).toBe(false);
  expect(sheet.secondHead).toEqual(["内部资料", "秩序册"]);
  expect(sheet.pageRule).toBe(true);
  expect(sheet.csp).toBe(true);
});

test("前置页的强制分页与超过一页的图片", async ({ page }) => {
  // A front page as Go renders its Markdown: a forced break inside a section
  // that fits one sheet, then a picture taller and wider than the sheet with
  // its caption in the same paragraph.
  const tall = `data:image/svg+xml,${encodeURIComponent("<svg xmlns='http://www.w3.org/2000/svg' width='1024' height='3000'><rect width='1024' height='3000' fill='%23888'/></svg>")}`;
  await installReport(page, document(`<section class="front"><h1>竞赛须知</h1><p>须知内容</p><div class="page-break"></div><h1>安全提示</h1><p>注意安全</p></section>
    <section class="front"><p><img src="${tall}" width="1024" height="3000" alt="徽标"><br>图后说明</p><p>下一段</p></section>
    <section class="event"><h2>男子100m</h2>${table(3, "a")}</section>`));
  await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);
  const frame = page.frameLocator('iframe[title="预览"]');
  const laid = await frame.locator("body").evaluate((body) => {
    const pages = [...body.querySelectorAll<HTMLElement>(".pv-page")];
    const pageOf = (text: string) => pages.findIndex((p) => p.querySelector(".pv-body")!.textContent!.includes(text));
    // Whether an element lies within its sheet's text area, not cut off.
    const shown = (el: Element | null) => {
      if (!el) return false;
      const box = el.getBoundingClientRect();
      const area = el.closest(".pv-body")!.getBoundingClientRect();
      return box.height > 0 && box.top >= area.top - 1 && box.bottom <= area.bottom + 1 && box.right <= area.right + 1;
    };
    const caption = [...body.querySelectorAll(".pv-body p")].find((p) => p.textContent?.includes("图后说明"));
    const range = document.createRange();
    const text = caption && [...caption.childNodes].find((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.includes("图后说明"));
    if (text) range.selectNode(text);
    const textBox = text ? range.getBoundingClientRect() : null;
    const area = caption?.closest(".pv-body")?.getBoundingClientRect();
    return {
      rules: pageOf("竞赛须知"), safety: pageOf("安全提示"),
      image: shown(body.querySelector(".pv-body img")),
      caption: !!(textBox && area && textBox.height > 0 && textBox.bottom <= area.bottom + 1),
      next: shown([...body.querySelectorAll(".pv-body p")].find((p) => p.textContent === "下一段") ?? null),
    };
  });
  expect(laid.safety).toBe(laid.rules + 1);
  expect(laid.image).toBe(true);
  expect(laid.caption).toBe(true);
  expect(laid.next).toBe(true);
});

test("前置页的图片段落与长文字不被截掉", async ({ page }) => {
  // Markdown's "![标识](asset:icon)", a blank line, then a paragraph: the
  // picture and its caption are two paragraphs. Then a front page of plain
  // paragraphs that takes more than one sheet.
  const tall = `data:image/svg+xml,${encodeURIComponent("<svg xmlns='http://www.w3.org/2000/svg' width='1024' height='1024'><rect width='1024' height='1024' fill='%23888'/></svg>")}`;
  const paragraphs = Array.from({ length: 60 }, (_, i) => `<p>第${i + 1}条 参赛须知的正文，内容较长，用于填满纸面并跨到下一页。</p>`).join("");
  await installReport(page, document(`<section class="front"><p><img src="${tall}" alt="标识"></p><p>图片之后的说明。</p></section>
    <section class="front"><h1>竞赛规程</h1>${paragraphs}</section>`));
  await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);
  const frame = page.frameLocator('iframe[title="预览"]');
  const laid = await frame.locator("body").evaluate((body) => {
    const shown = (el: Element | undefined) => {
      if (!el) return false;
      const box = el.getBoundingClientRect();
      const area = el.closest(".pv-body")!.getBoundingClientRect();
      return box.height > 0 && box.top >= area.top - 1 && box.bottom <= area.bottom + 1;
    };
    const ps = [...body.querySelectorAll(".pv-body p")];
    const numbered = ps.filter((p) => /^第\d+条/.test(p.textContent ?? ""));
    return {
      caption: shown(ps.find((p) => p.textContent === "图片之后的说明。")),
      image: shown(body.querySelector(".pv-body img") ?? undefined),
      paragraphs: numbered.length,
      hidden: numbered.filter((p) => !shown(p)).map((p) => p.textContent!.slice(0, 4)),
    };
  });
  expect(laid.image).toBe(true);
  expect(laid.caption).toBe(true);
  expect(laid.paragraphs).toBe(60);
  expect(laid.hidden).toEqual([]);
});
