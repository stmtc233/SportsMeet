import { expect, test, type Page } from "@playwright/test";
import { installRuntime, selectReport } from "./runtime";

// A bridge for the print style pages: the styles, their images, the meet's
// front pages and the meet's choice, kept in memory. Go's checks and the
// rendering of reports are Go tests; this is about the pages.
async function installStyles(page: Page, options: { warnings?: string[] } = {}) {
  await installRuntime(page);
  await page.addInitScript((options) => {
    const bridge = (globalThis as any).mygo;
    const base = bridge.call;
    const baseOn = bridge.on;
    const listeners = new Set<(c: unknown) => void>();
    bridge.on = (name: string, fn: (c: unknown) => void) => { listeners.add(fn); const off = baseOn(name, fn); return () => { listeners.delete(fn); off(); }; };
    const emit = (kind: string, meetId = "") => listeners.forEach((fn) => fn({ kind, meetId, eventId: "" }));
    const clone = (v: unknown) => structuredClone(v);
    const calls: { method: string; args: unknown[] }[] = (globalThis as any).__styleCalls = [];
    (globalThis as any).__emit = emit;
    (globalThis as any).__renderWarnings = options.warnings ?? [];

    const base0 = { paper: "a4", paperWidth: 0, paperHeight: 0, orientation: "portrait", marginTop: 15, marginRight: 15, marginBottom: 15, marginLeft: 15,
      font: "sans", fontSize: 9.5, lineHeight: 1.4, titleSize: 18, eventTitleSize: 12.5, tableLines: "rules", lineWidth: 1,
      header: true, pageNumber: "n_of_m", rosterColumns: 4, showUnit: true, showSeed: true, showBib: true };
    let styles: any[] = [{ id: "s1", name: "校内赛", version: 2, usedBy: 2, hasCustomCss: true, hasTemplates: false, params: { ...base0 },
      assets: [{ name: "logo", mime: "image/png", sha256: "aa", size: 2048 }], meets: [{ id: "meet", name: "秋季运动会" }] }];
    let meetStyle = "";
    let meetAssets: any[] = [];
    let pages: any[] = [];
    let next = 1;
    const used = (id: string) => (id === "default" ? 1 : styles.find((s) => s.id === id)?.usedBy ?? 0);
    const summary = (s: any) => ({ id: s.id, name: s.name, builtin: false, version: s.version, updatedAt: 1, usedBy: used(s.id), hasCustomCss: s.hasCustomCss, hasTemplates: s.hasTemplates });
    const builtin = { id: "default", name: "默认", builtin: true, version: 0, updatedAt: 0, usedBy: 1, hasCustomCss: false, hasTemplates: false };
    const detail = (id: string) => {
      if (id === "default") return { ...builtin, createdAt: 0, params: { ...base0 }, css: "", slots: {}, templates: {}, dataVersion: 1, assets: [], meets: [] };
      const s = styles.find((x) => x.id === id);
      if (!s) throw Object.assign(new Error("打印样式不存在或已删除"), { name: "CallError" });
      return { ...summary(s), createdAt: 1, params: clone(s.params), css: "", slots: {}, templates: {}, dataVersion: 1, assets: clone(s.assets), meets: clone(s.meets) };
    };
    const saved = (id: string, warnings: string[] = []) => ({ style: detail(id), version: styles.find((s) => s.id === id)?.version ?? 1, changed: true, warnings });
    const fail = (message: string) => { throw Object.assign(new Error(message), { name: "CallError" }); };
    const png = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==";

    bridge.call = async (method: string, ...args: any[]) => {
      calls.push({ method, args: clone(args) as unknown[] });
      switch (method) {
        case "PrintStyles.List": return [builtin, ...styles.map(summary)];
        case "PrintStyles.Get": return detail(args[0]);
        case "PrintStyles.Save": {
          const s = styles.find((x) => x.id === args[0])!;
          if (args[1].params) s.params = clone(args[1].params);
          s.version++;
          emit("printStyles");
          return saved(s.id);
        }
        case "PrintStyles.Rename": {
          if (styles.some((x) => x.id !== args[0] && x.name === args[1])) return fail("已有名为「" + args[1] + "」的样式");
          const s = styles.find((x) => x.id === args[0])!;
          s.name = args[1]; s.version++;
          emit("printStyles");
          return saved(s.id);
        }
        case "PrintStyles.Copy": {
          const from = args[0] === "default" ? { name: "默认", params: base0, assets: [], hasCustomCss: false, hasTemplates: false } : styles.find((x) => x.id === args[0])!;
          const id = `s${++next}`;
          styles.push({ id, name: `${from.name} 副本`, version: 1, usedBy: 0, hasCustomCss: from.hasCustomCss, hasTemplates: from.hasTemplates, params: clone(from.params), assets: clone(from.assets), meets: [] });
          emit("printStyles");
          return saved(id);
        }
        case "PrintStyles.Delete": styles = styles.filter((x) => x.id !== args[0]); if (meetStyle === args[0]) meetStyle = ""; emit("printStyles"); return null;
        case "PrintStyles.Versions": return [
          { version: 2, current: true, at: Date.UTC(2026, 10, 12, 1, 30), atText: "2026-11-12 09:30", source: "ui", reason: "保存", name: "校内赛", hasCustomCss: true, hasTemplates: false, assets: 1 },
          { version: 1, current: false, at: Date.UTC(2026, 10, 11, 1, 30), atText: "2026-11-11 09:30", source: "mcp", reason: "新建", name: "校内赛", hasCustomCss: true, hasTemplates: false, assets: 0 },
        ];
        case "PrintStyles.Restore": { const s = styles.find((x) => x.id === args[0])!; s.version++; emit("printStyles"); return saved(s.id); }
        case "PrintStyles.Assets": return args[0] === "meet" ? clone(meetAssets) : clone(styles.find((x) => x.id === args[1])?.assets ?? []);
        case "PrintStyles.AssetImage": return png;
        case "PrintStyles.UploadAsset": {
          const a = { name: "新图", mime: "image/png", sha256: "bb", size: 300 };
          if (args[0] === "meet") { meetAssets.push(a); emit("meets", args[1]); } else { styles.find((x) => x.id === args[1])!.assets.push(a); emit("printStyles"); }
          return a;
        }
        case "PrintStyles.DeleteAsset":
          if (args[0] === "meet") { meetAssets = meetAssets.filter((a) => a.name !== args[2]); emit("meets", args[1]); }
          else { const s = styles.find((x) => x.id === args[1])!; s.assets = s.assets.filter((a: any) => a.name !== args[2]); emit("printStyles"); }
          return null;
        case "PrintStyles.Import": return null;
        case "PrintStyles.Export": return "/tmp/校内赛.smstyle.json";
        case "FrontPages.List": return clone(pages).map((p: any, i: number) => ({ ...p, position: i + 1 }));
        case "FrontPages.Save": {
          const [meetId, id, input] = args;
          if (!input.title.trim()) return fail("前置页标题需要 1–60 个字");
          const row = { id: id || `p${++next}`, meetId, position: 0, title: input.title, bodyMd: input.bodyMd, reports: input.reports, updatedAt: 1 };
          pages = id ? pages.map((p) => (p.id === id ? row : p)) : [...pages, row];
          emit("meets", meetId);
          const missing = [...input.bodyMd.matchAll(/asset:([^\s)]+)/g)].map((m) => m[1]).filter((n) => !meetAssets.some((a) => a.name === n) && !styles.find((s) => s.id === meetStyle)?.assets.some((a: any) => a.name === n));
          return { page: row, warnings: missing.map((n) => `找不到图片「${n}」，请先上传到赛事图片`) };
        }
        case "FrontPages.Delete": pages = pages.filter((p) => p.id !== args[1]); emit("meets", args[0]); return null;
        case "FrontPages.Reorder": pages = args[1].map((id: string) => pages.find((p) => p.id === id)); emit("meets", args[0]); return null;
        case "Meets.SetPrintStyle": meetStyle = args[1] === "default" ? "" : args[1]; emit("meets", args[0]); return null;
        case "Meets.Settings": return { ...(await base(method, ...args)), printStyleId: meetStyle };
        case "Reports.Render": {
          const html = `<!doctype html><html><body><h1>秋季运动会</h1><p>样式 ${meetStyle || "default"}</p></body></html>`;
          return { html, warnings: (globalThis as any).__renderWarnings };
        }
        default: return base(method, ...args);
      }
    };
  }, options);
}

const calls = (page: Page, method: string) => page.evaluate((m) => (globalThis as any).__styleCalls.filter((c: any) => c.method === m).map((c: any) => c.args), method);
const overflow = (page: Page) => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth);

async function openStyles(page: Page, width: number) {
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: "打印样式", exact: true }).click();
  await expect(page.getByRole("heading", { name: "打印样式", exact: true })).toBeVisible();
  // A narrow window shows the styles first, then the chosen one's page.
  if (width <= 900) await expect(page.getByLabel("字体")).toBeHidden();
}

const styleList = (page: Page) => page.getByRole("navigation", { name: "打印样式" });

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`打印样式页 ${width} ${colorScheme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      await installStyles(page);
      await openStyles(page, width);

      // The built-in style is first, marked, and read-only.
      await expect(styleList(page).getByRole("button")).toHaveCount(2);
      await expect(styleList(page).getByRole("button").first()).toContainText("默认");
      await expect(styleList(page).getByRole("button").first()).toContainText("内置");
      if (width > 900) await expect(page.getByLabel("正文字号")).toBeDisabled();
      else await styleList(page).getByRole("button").first().click();
      await expect(page.getByLabel("正文字号")).toBeDisabled();
      await expect(page.getByRole("button", { name: "删除", exact: true })).toBeDisabled();
      await expect(page.getByLabel("名称")).toBeDisabled();
      if (width <= 900) await page.getByRole("button", { name: "样式列表", exact: true }).click();

      await styleList(page).getByRole("button", { name: /校内赛/ }).click();
      await expect(page.getByLabel("名称")).toHaveValue("校内赛");
      await expect(page.getByText("含自定义 CSS", { exact: true })).toBeVisible();
      await expect(page.getByText("含自定义模板", { exact: true })).toHaveCount(0);

      // A wrong number says so at its field and is not saved.
      const size = page.getByLabel("正文字号");
      await size.fill("5");
      await expect(page.getByText("正文字号需在 6–24 pt 之间", { exact: true })).toBeVisible();
      await expect(size).toHaveAttribute("aria-invalid", "true");
      expect(await calls(page, "PrintStyles.Save")).toEqual([]);
      // A good one is saved at once, with every parameter.
      await size.fill("10.5");
      await expect(page.getByText("正文字号需在", { exact: false })).toHaveCount(0);
      await expect.poll(async () => (await calls(page, "PrintStyles.Save")).length).toBe(1);
      expect((await calls(page, "PrintStyles.Save"))[0]).toMatchObject(["s1", { name: null, params: { fontSize: 10.5, paper: "a4", marginTop: 15 } }]);
      await size.blur();
      await expect(size).toHaveValue("10.5");
      await page.screenshot({ path: testInfo.outputPath("styles.png"), animations: "disabled" });

      // Margins that leave no room are caught before Go is asked.
      await page.getByLabel("纸张", { exact: true }).click();
      await page.getByRole("option", { name: "自定义" }).click();
      await expect(page.getByLabel("纸张宽")).toHaveValue("210");
      await expect.poll(async () => (await calls(page, "PrintStyles.Save")).length).toBe(2);
      await page.getByLabel("纸张宽").fill("100");
      await expect.poll(async () => (await calls(page, "PrintStyles.Save")).length).toBe(3);
      await page.getByLabel("左边距").fill("50");
      await expect(page.getByText("边距太大，纸上放不下内容", { exact: true })).toBeVisible();
      expect((await calls(page, "PrintStyles.Save")).length).toBe(3);
      await page.getByLabel("左边距").fill("10");

      // A switch is a save as well.
      const unit = page.getByRole("switch", { name: "单位", exact: true });
      await expect(unit).toHaveAttribute("aria-checked", "true");
      await unit.click();
      await expect(unit).toHaveAttribute("aria-checked", "false");
      await expect.poll(async () => (await calls(page, "PrintStyles.Save")).at(-1)?.[1].params.showUnit).toBe(false);
      expect(await overflow(page)).toBe(true);
    });
  }
}

test("复制、改名、导出与删除确认", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installStyles(page);
  await openStyles(page, 1280);
  await styleList(page).getByRole("button", { name: /校内赛/ }).click();

  await page.getByRole("button", { name: "复制", exact: true }).click();
  // The copy is chosen and named after its source.
  await expect(page.getByLabel("名称")).toHaveValue("校内赛 副本");
  await expect(styleList(page).getByRole("button")).toHaveCount(3);

  const name = page.getByLabel("名称");
  await name.fill("决赛日");
  await name.press("Enter");
  await expect(styleList(page).getByRole("button", { name: /决赛日/ })).toBeVisible();
  expect((await calls(page, "PrintStyles.Rename")).at(-1)).toEqual(["s2", "决赛日"]);

  await page.getByRole("button", { name: "导出", exact: true }).click();
  await expect.poll(async () => (await calls(page, "PrintStyles.Export")).length).toBe(1);

  // Deleting says how many meets use the style.
  await styleList(page).getByRole("button", { name: /校内赛/ }).click();
  await page.getByRole("button", { name: "删除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "删除「校内赛」？" });
  await expect(dialog).toContainText("2 个赛事正在使用，删除后改用「默认」。");
  await dialog.getByRole("button", { name: "取消" }).click();
  expect(await calls(page, "PrintStyles.Delete")).toEqual([]);
  await page.getByRole("button", { name: "删除", exact: true }).click();
  await dialog.getByRole("button", { name: "删除", exact: true }).click();
  await expect(styleList(page).getByRole("button", { name: /校内赛/ })).toHaveCount(0);
  await expect(page.getByLabel("名称")).toHaveValue("默认");
});

test("历史版本、图片", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installStyles(page);
  await openStyles(page, 1280);
  await styleList(page).getByRole("button", { name: /校内赛/ }).click();

  await page.getByRole("button", { name: "历史版本", exact: true }).click();
  const versions = page.getByRole("dialog", { name: "历史版本" });
  await expect(versions).toContainText("界面");
  await expect(versions).toContainText("AI");
  await expect(versions).toContainText("2026-11-12 09:30");
  await expect(versions.getByText("当前", { exact: true })).toBeVisible();
  await versions.getByRole("button", { name: "恢复", exact: true }).click();
  await expect.poll(async () => (await calls(page, "PrintStyles.Restore")).length).toBe(1);
  expect((await calls(page, "PrintStyles.Restore"))[0]).toEqual(["s1", 1]);
  await expect(versions).toHaveCount(0);

  await expect(page.getByText("asset:logo · 2 KB", { exact: true })).toBeVisible();
  await expect(page.locator("img.asset-thumb")).toHaveCount(1);
  await page.getByRole("button", { name: "上传", exact: true }).click();
  await expect(page.getByText("asset:新图 · 300 B", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "删除图片 logo" }).click();
  await page.getByRole("dialog", { name: "删除图片「logo」？" }).getByRole("button", { name: "删除", exact: true }).click();
  await expect(page.getByText("asset:logo", { exact: false })).toHaveCount(0);
});

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`赛事设置的打印样式与前置页 ${width} ${colorScheme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      await installStyles(page);
      await page.goto("/");
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "赛事设置", exact: true }).click();

      // The style is the meet's choice, at once.
      await page.getByLabel("打印样式", { exact: true }).click();
      await page.getByRole("option", { name: "校内赛" }).click();
      await expect.poll(async () => (await calls(page, "Meets.SetPrintStyle")).length).toBe(1);
      expect((await calls(page, "Meets.SetPrintStyle"))[0]).toEqual(["meet", "s1"]);

      // Front pages: a new one whose text refers to a missing image.
      await page.getByRole("button", { name: "新建", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "新建前置页" });
      await dialog.getByLabel("标题").fill("致辞");
      await dialog.getByLabel("正文").fill("# 欢迎\n\n![](asset:徽标)");
      await dialog.getByRole("button", { name: "保存", exact: true }).click();
      await expect(dialog.getByText("找不到图片「徽标」，请先上传到赛事图片", { exact: true })).toBeVisible();
      await page.waitForTimeout(300); // the notice fades in
      await page.screenshot({ path: testInfo.outputPath("front-page.png"), animations: "disabled" });
      // The page is saved: the text can be fixed and saved again, not added twice.
      await dialog.getByLabel("正文").fill("# 欢迎");
      await dialog.getByRole("button", { name: "保存", exact: true }).click();
      await expect(dialog).toHaveCount(0);
      expect((await calls(page, "FrontPages.Save")).map((a: any) => a[1])).toEqual(["", "p2"]);
      await expect(page.getByText("印在全部报表前", { exact: true })).toBeVisible();

      // Another page, in front of the 秩序册 only; the order can be changed.
      await page.getByRole("button", { name: "新建", exact: true }).click();
      await dialog.getByLabel("标题").fill("须知");
      await dialog.getByRole("button", { name: "印在哪些报表前" }).click();
      await page.getByRole("menuitemcheckbox", { name: "秩序册" }).click();
      await page.keyboard.press("Escape");
      await dialog.getByRole("button", { name: "保存", exact: true }).click();
      await expect(page.getByText("印在秩序册前", { exact: true })).toBeVisible();
      await page.getByRole("button", { name: "须知 更多", exact: true }).click();
      await page.getByRole("menuitem", { name: "上移" }).click();
      await expect.poll(async () => (await calls(page, "FrontPages.Reorder")).length).toBe(1);
      expect((await calls(page, "FrontPages.Reorder"))[0]).toEqual(["meet", ["p3", "p2"]]);

      // Deleting asks first.
      await page.getByRole("button", { name: "致辞 更多", exact: true }).click();
      await page.getByRole("menuitem", { name: "删除" }).click();
      await page.getByRole("dialog", { name: "删除「致辞」？" }).getByRole("button", { name: "删除", exact: true }).click();
      await expect(page.getByText("致辞", { exact: true })).toHaveCount(0);

      await expect(page.getByRole("heading", { name: "赛事图片" })).toBeVisible();
      expect(await overflow(page)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath("setup.png"), animations: "disabled" });

      // 管理样式 opens the style the meet uses, and back returns here.
      await page.getByRole("button", { name: "管理样式", exact: true }).click();
      await expect(page.getByLabel("名称")).toHaveValue("校内赛");
      await page.getByRole("button", { name: "返回", exact: true }).click();
      await expect(page.getByRole("heading", { name: "赛事设置", exact: true })).toBeVisible();
    });
  }
}

test("打印预览显示样式的提示，样式变化后重新读取", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installStyles(page, { warnings: ["自定义模板「startlist」无法使用，已改用默认版式", "找不到图片「徽标」", "第三条", "第四条"] });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await selectReport(page, "秩序册");
  await expect(page.getByText("自定义模板「startlist」无法使用，已改用默认版式", { exact: true })).toBeVisible();
  await expect(page.getByText("另有 1 条提示", { exact: true })).toBeVisible();
  await expect(page.locator(".subbar__count")).toHaveText(/\d+ 页/);

  // An AI saving a style (no meet in the event) reads the preview again.
  const before = (await calls(page, "Reports.Render")).length;
  await page.evaluate(() => { (globalThis as any).__renderWarnings = []; (globalThis as any).__emit("printStyles"); });
  await expect.poll(async () => (await calls(page, "Reports.Render")).length).toBeGreaterThan(before);
  await expect(page.getByText("另有 1 条提示", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("status")).toHaveCount(0);
});
