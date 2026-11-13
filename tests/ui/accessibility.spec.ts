import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function openMeet(page: Page, count = 24) {
  await installRuntime(page, { athleteCount: count });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
}

for (const width of [1280, 800]) {
  test(`下拉选择与单选组键盘操作 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await openMeet(page);
    await page.getByRole("button", { name: "添加项目", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "添加项目", exact: true });
    const select = dialog.getByLabel("常用项目", { exact: true });
    await select.focus();
    await select.press("ArrowDown");
    const list = page.getByRole("listbox");
    await expect(list).toHaveAttribute("id", (await select.getAttribute("aria-controls"))!);
    await expect(list.getByRole("option", { name: "自定义", exact: true })).toBeFocused();
    await page.keyboard.press("End");
    await expect(list.getByRole("option", { name: "接力 · 混合4×400m", exact: true })).toBeFocused();
    await page.keyboard.press("Home");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Enter");
    await expect(dialog.getByLabel("项目名", { exact: true })).toHaveValue("1500m");
    await expect(select).toBeFocused();
    await select.press("Enter");
    await expect(list.getByRole("option", { name: "中长跑 · 1500m", exact: true })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(list).not.toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(select).toBeFocused();
    await select.press("Space");
    await page.keyboard.press("Tab");
    await expect(list).not.toBeVisible();
    const group = dialog.getByRole("radiogroup", { name: "性别", exact: true });
    await expect(group.getByRole("radio", { name: "男", exact: true })).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await expect(group.getByRole("radio", { name: "女", exact: true })).toBeChecked();
    await expect(group.getByRole("radio", { name: "女", exact: true })).toBeFocused();
    await expect(group.locator('[tabindex="0"]')).toHaveCount(1);
    await page.keyboard.press("Tab");
    await expect(dialog.getByLabel("项目名", { exact: true })).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(group.getByRole("radio", { name: "女", exact: true })).toBeFocused();
    await page.keyboard.press("End");
    await expect(group.getByRole("radio", { name: "混合", exact: true })).toBeChecked();
  });

  test(`表格勾选、键盘编辑与虚拟行信息 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 500 });
    await openMeet(page, 1000);
    await page.getByRole("button", { name: "运动员", exact: true }).click();
    const table = page.getByRole("table");
    await expect(table).toHaveAttribute("aria-rowcount", "1001");
    await expect(table.getByRole("row").nth(1)).toHaveAttribute("aria-rowindex", "2");
    await table.getByRole("checkbox", { name: "选择运动员1", exact: true }).check();
    const all = table.getByRole("checkbox", { name: "全选", exact: true });
    await expect(all).toBeChecked({ indeterminate: true });
    await all.check();
    await expect(all).toBeChecked();
    expect(await all.evaluate((el: HTMLInputElement) => el.indeterminate)).toBe(false);
    await all.uncheck();
    const edit = table.getByRole("button", { name: "运动员1", exact: true });
    await edit.focus();
    await edit.press("Enter");
    await expect(page.getByRole("dialog", { name: "编辑运动员", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(table.getByRole("button", { name: "运动员1000", exact: true })).toBeVisible();
    await expect(table.getByRole("row").last()).toHaveAttribute("aria-rowindex", "1001");
    expect(await table.getByRole("row").count()).toBeLessThan(100);
  });
}

test("操作菜单跳过禁用项，Tab 离开并返回正常顺序", async ({ page }) => {
  await openMeet(page);
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
  await page.getByRole("button", { name: "添加组", exact: true }).click();
  const more = page.locator(".subbar").getByRole("button", { name: "更多", exact: true });
  await more.focus();
  await more.press("ArrowDown");
  const menu = page.getByRole("menu");
  await expect(menu.getByRole("menuitem", { name: "批量排时间…", exact: true })).toBeDisabled();
  await expect(menu.getByRole("menuitem", { name: "全部重置…", exact: true })).toBeFocused();
  await page.keyboard.press("Home");
  await page.keyboard.press("ArrowDown");
  await expect(menu.getByRole("menuitem", { name: "全部重置…", exact: true })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(more).toBeFocused();
  await more.press("Enter");
  await page.keyboard.press("Tab");
  await expect(menu).not.toBeVisible();
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeFocused();
});

test("错误状态、说明关联与延后播报", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "MCP", exact: true }).click();
  const port = page.getByLabel("端口", { exact: true });
  await port.fill("1");
  await expect(port).toHaveAttribute("aria-invalid", "true");
  await expect(port).toHaveAccessibleDescription("请输入 1024 至 65535");
  await port.press("Tab");
  const announcement = page.locator('.st-sr-only[aria-live="polite"]').first();
  await expect(announcement).toHaveText("端口：请输入 1024 至 65535");
  await port.fill("18766");
  await expect(port).toHaveAttribute("aria-invalid", "false");
  await expect(port).not.toHaveAttribute("aria-describedby");
  await expect(announcement).toHaveText("");
});

test("对话框提交失败后聚焦错误字段，修正后可继续", async ({ page }) => {
  await installRuntime(page, { eventConfig: "height" });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子跳高", { exact: true }).click();
  await page.getByRole("radio", { name: "成绩", exact: true }).click();
  const add = page.getByRole("textbox", { name: "高度", exact: true });
  await add.fill("1.50");
  await add.press("Enter");
  await add.fill("1.55");
  await add.press("Enter");
  await page.getByRole("button", { name: "高度 1.50", exact: true }).click();
  await page.getByRole("menuitem", { name: "修改高度", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "修改高度", exact: true });
  const height = dialog.getByLabel("高度", { exact: true });
  await height.fill("1.55");
  await dialog.getByRole("button", { name: "确定", exact: true }).click();
  await expect(height).toHaveAccessibleDescription("该高度已存在");
  await expect(height).toBeFocused();
  await height.fill("1.60");
  await expect(height).toHaveAttribute("aria-invalid", "false");
  await height.press("Enter");
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole("columnheader", { name: "1.60 m", exact: true })).toBeVisible();
});

function contrast(a: number[], b: number[]) {
  const luminance = (rgb: number[]) => rgb.reduce((sum, v, i) => {
    const c = v / 255;
    return sum + (c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][i]!;
  }, 0);
  const x = luminance(a), y = luminance(b);
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05);
}

for (const colorScheme of ["light", "dark"] as const) {
  test(`主题文字与焦点对比度 ${colorScheme}`, async ({ page }) => {
    await page.emulateMedia({ colorScheme });
    await installRuntime(page);
    await page.goto("/");
    const primary = page.getByRole("button", { name: "新建赛事", exact: true });
    const colors = () => primary.evaluate((el) => {
      const style = getComputedStyle(el);
      const rgb = (s: string) => s.match(/[\d.]+/g)!.slice(0, 3).map(Number);
      return { text: rgb(style.color), background: rgb(style.backgroundColor), focus: rgb(style.outlineColor), surface: rgb(getComputedStyle(el.closest("header")!).backgroundColor) };
    });
    // Check both system-following and explicit theme selectors.
    for (const theme of ["system", colorScheme]) {
      await page.evaluate((theme) => { document.documentElement.dataset.theme = theme; }, theme);
      await primary.focus();
      let c = await colors();
      expect(contrast(c.text, c.background)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(c.focus, c.surface)).toBeGreaterThanOrEqual(3);
      await primary.hover();
      // CSS colors transition; sample after the declared motion duration.
      await expect.poll(async () => { c = await colors(); return contrast(c.text, c.background); }).toBeGreaterThanOrEqual(4.5);
      await page.mouse.move(0, 0);
    }
    await page.screenshot({ path: `test-results/accessibility-${colorScheme}.png`, animations: "disabled" });
  });
}

test("强制颜色保留焦点和选中轮廓", async ({ page }) => {
  await page.emulateMedia({ forcedColors: "active" });
  await installRuntime(page);
  await page.goto("/");
  const primary = page.getByRole("button", { name: "新建赛事", exact: true });
  await primary.focus();
  await expect(primary).toHaveCSS("outline-style", "solid");
  await expect(primary).toHaveCSS("outline-width", "2px");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "通用", exact: true }).click();
  await expect(page.getByRole("radio", { name: "跟随系统", exact: true })).toHaveCSS("outline-width", "2px");
  await page.screenshot({ path: "test-results/accessibility-forced-colors.png", animations: "disabled" });
});
