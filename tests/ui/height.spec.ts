import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function openHeight(page: Page, options: { heightRounds?: boolean; rejectHeightSave?: boolean } = {}) {
  await installRuntime(page, { eventConfig: "height", ...options });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "项目", exact: true }).click();
  await page.getByText("男子跳高", { exact: true }).click();
  await page.getByRole("radio", { name: "成绩", exact: true }).click();
}
async function addHeight(page: Page, value = "1.50") {
  await page.getByRole("textbox", { name: "高度", exact: true }).fill(value);
  await page.getByRole("textbox", { name: "高度", exact: true }).press("Enter");
  await expect(page.getByRole("columnheader", { name: `${value} m`, exact: false })).toBeVisible();
}

async function typeTrials(page: Page, field: ReturnType<Page["getByRole"]>, value: string) {
  await field.press("Enter");
  await field.fill(value);
}

for (const width of [1280, 800]) for (const colorScheme of ["light", "dark"] as const) {
  test(`高度单键录入、预览、过程保存与结束 ${width} ${colorScheme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await page.emulateMedia({ colorScheme });
    await openHeight(page);
    await expect(page.getByText("添加起跳高度", { exact: true })).toBeVisible();
    const head = page.getByRole("table", { name: "第1组高度成绩" }).getByRole("row").first();
    const [order, name] = await Promise.all([head.getByText("顺序").boundingBox(), head.getByText("姓名／单位").boundingBox()]);
    expect(Math.abs(order!.y - name!.y)).toBeLessThan(4);
    expect(name!.x).toBeGreaterThan(order!.x);
    await expect(page.getByRole("button", { name: "成功 O", exact: true })).toHaveCount(0);
    await addHeight(page);
    const trials = page.getByRole("textbox", { name: "1.50 m试跳", exact: true });
    await expect(trials.nth(0)).toBeFocused();
    await expect(page.getByRole("button", { name: "成功 O", exact: true })).toHaveCount(1);
    const [cell, button] = await Promise.all([trials.nth(0).boundingBox(), page.getByRole("button", { name: "成功 O", exact: true }).boundingBox()]);
    expect(Math.abs(cell!.y - button!.y)).toBeLessThan(12);
    expect(button!.x - cell!.x).toBeLessThan(160);
    await expect(trials.nth(0)).toHaveValue("");
    await page.getByRole("button", { name: "失败 X", exact: true }).click();
    await expect(trials.nth(0)).toHaveValue("X");
    await expect(trials.nth(0)).toBeFocused();
    await expect(page.getByText("第 2 次", { exact: true })).toBeVisible();
    await trials.nth(0).press("x");
    await expect(trials.nth(0)).toHaveValue("XX");
    await page.getByRole("button", { name: "撤销一次", exact: true }).click();
    await expect(trials.nth(0)).toHaveValue("X");
    await expect(trials.nth(0)).toBeFocused();
    await trials.nth(0).press("x");
    await trials.nth(0).press("Backspace");
    await expect(trials.nth(0)).toHaveValue("X");
    await trials.nth(0).press("ArrowDown");
    await expect(trials.nth(1)).toBeFocused();
    await expect(page.getByRole("button", { name: "撤销一次", exact: true })).toBeDisabled();
    await trials.nth(1).press("o");
    await expect(trials.nth(1)).toHaveValue("O");
    await expect(trials.nth(1)).toBeFocused();
    await trials.nth(1).press("ArrowDown");
    await trials.nth(2).press("-");
    await expect(trials.nth(2)).toHaveValue("-");
    await trials.nth(0).click();
    await page.getByRole("button", { name: "成功 O", exact: true }).click();
    await expect(trials.nth(0)).toHaveValue("XO");
    await expect(trials.nth(0)).toBeFocused();
    const add = page.getByRole("textbox", { name: "高度", exact: true });
    await expect(add).toHaveAttribute("placeholder", "1.55");
    await add.press("Enter");
    const next = page.getByRole("textbox", { name: "1.55 m试跳", exact: true });
    await expect(next.nth(0)).toBeFocused();
    await expect(page.getByText("当前最好 1.50", { exact: true })).toBeVisible();
    await next.nth(0).press("o");
    await next.nth(0).press("ArrowDown");
    await expect(next.nth(1)).toBeFocused();
    const table = page.getByRole("table", { name: "第1组高度成绩" });
    await table.getByRole("button", { name: "状态", exact: true }).nth(2).click();
    await page.getByRole("menuitem", { name: "弃权", exact: true }).click();
    await expect(table.getByText("弃权", { exact: true })).toBeVisible();
    await expect(next.nth(2)).toBeDisabled();
    await next.nth(1).press("x");
    await expect(next.nth(1)).toBeFocused();
    await typeTrials(page, next.nth(1), "XXX");
    const save = page.getByRole("button", { name: "保存", exact: true });
    await expect(save).toBeEnabled();
    await save.click();
    await expect(save).toBeDisabled();
    await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeDisabled();
    await expect(page.getByText("已录完", { exact: true })).toHaveCount(0);
    await page.getByRole("button", { name: "结束录入", exact: true }).click();
    await page.getByRole("dialog", { name: "结束这一组录入？" }).getByRole("button", { name: "结束录入", exact: true }).click();
    await expect(page.getByText("已结束", { exact: true })).toBeVisible();
    await expect(page.getByText("已录完", { exact: true })).toBeVisible();
    await typeTrials(page, trials.nth(0), "XXO");
    await expect(save).toBeEnabled();
    await save.click();
    await expect(page.getByText("已结束", { exact: true })).toHaveCount(0);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: `/tmp/sportsmeet-height-${width}-${colorScheme}.png` });
  });
}

test("空高度可撤销、重复高度提示、移除有记录列需确认", async ({ page }) => {
  await openHeight(page);
  await addHeight(page);
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "放弃", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "1.50 m试跳", exact: true })).toHaveCount(0);
  await addHeight(page);
  await page.getByRole("textbox", { name: "高度", exact: true }).fill("1.50");
  await page.getByRole("button", { name: "添加高度", exact: true }).click();
  await expect(page.getByText("该高度已存在", { exact: true })).toBeVisible();
  await page.getByRole("textbox", { name: "高度", exact: true }).fill("");
  await page.getByRole("textbox", { name: "1.50 m试跳", exact: true }).nth(0).press("o");
  await page.getByRole("button", { name: "高度 1.50", exact: true }).click();
  await page.getByRole("menuitem", { name: "移除高度", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "移除高度？" })).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "1.50 m试跳", exact: true }).nth(0)).toHaveValue("O");
  await page.getByRole("button", { name: "高度 1.50", exact: true }).click();
  await page.getByRole("menuitem", { name: "移除高度", exact: true }).click();
  await page.getByRole("button", { name: "移除", exact: true }).click();
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeDisabled();
});

test("试跳按文本校验、方向键跳过弃权、外部刷新与轮次切换保留草稿", async ({ page }) => {
  await openHeight(page, { heightRounds: true });
  await addHeight(page);
  const trials = page.getByRole("textbox", { name: "1.50 m试跳", exact: true });
  await trials.nth(0).press("1");
  await expect(trials.nth(0)).toHaveValue("");
  await typeTrials(page, trials.nth(0), "1");
  await trials.nth(0).press("Tab");
  await expect(trials.nth(0)).toHaveValue("1");
  await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
  await expect(page.getByRole("alert")).toContainText("试跳记录不正确");
  await expect(trials.nth(0)).toHaveAttribute("aria-invalid", "true");
  await typeTrials(page, trials.nth(0), "XO");
  await trials.nth(0).press("Enter");
  const table = page.getByRole("table", { name: "第1组高度成绩" });
  await table.getByRole("button", { name: "状态", exact: true }).nth(1).click();
  await page.getByRole("menuitem", { name: "弃权", exact: true }).click();
  await trials.nth(0).focus();
  await trials.nth(0).press("ArrowDown");
  await expect(trials.nth(2)).toBeFocused();
  await trials.nth(2).press("ArrowUp");
  await expect(trials.nth(0)).toBeFocused();
  await page.evaluate(() => (globalThis as any).__scheduleExternalChange());
  await page.getByRole("radio", { name: "半决赛", exact: true }).click();
  await expect(trials).toHaveCount(0);
  await page.getByRole("radio", { name: "决赛", exact: true }).click();
  await expect(trials.nth(0)).toHaveValue("XO");
  await expect(page.getByRole("button", { name: "保存", exact: true })).toBeEnabled();
  await page.getByRole("radio", { name: "报名", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "放弃未保存的改动？" })).toBeVisible();
});

test("高度保存失败保留输入", async ({ page }) => {
  await openHeight(page, { rejectHeightSave: true });
  await addHeight(page);
  const field = page.getByRole("textbox", { name: "1.50 m试跳", exact: true }).nth(0);
  await typeTrials(page, field, "XO");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("保存失败，请重试", { exact: true })).toBeVisible();
  await expect(field).toHaveValue("XO");
  await expect(page.getByRole("button", { name: "保存", exact: true })).toBeEnabled();
});

test("修改已保存高度保留记录、放弃恢复、保存后移除可清空成绩", async ({ page }) => {
  await openHeight(page);
  await addHeight(page);
  await page.getByRole("textbox", { name: "1.50 m试跳", exact: true }).nth(0).press("o");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "高度 1.50", exact: true }).click();
  await page.getByRole("menuitem", { name: "修改高度", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "修改高度", exact: true });
  await dialog.getByLabel("高度", { exact: true }).fill("1.55");
  await dialog.getByRole("button", { name: "确定", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "1.55 m试跳", exact: true }).nth(0)).toHaveValue("O");
  await page.getByRole("button", { name: "放弃", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "1.50 m试跳", exact: true }).nth(0)).toHaveValue("O");
  await page.getByRole("button", { name: "高度 1.50", exact: true }).click();
  await page.getByRole("menuitem", { name: "移除高度", exact: true }).click();
  await page.getByRole("dialog", { name: "移除高度？" }).getByRole("button", { name: "移除", exact: true }).click();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("button", { name: "放弃", exact: true })).toBeDisabled();
  await expect(page.getByRole("columnheader", { name: "1.50 m" })).toHaveCount(0);
});
