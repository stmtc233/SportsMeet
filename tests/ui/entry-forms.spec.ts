import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`报名页导出与导入登记表 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");

    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "报名", exact: true }).click();
    await expect(page.getByText("1 / 2 已报名", { exact: true })).toBeVisible();
    await expect(page.getByText("未报名", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    await page.getByRole("button", { name: "导出登记表", exact: true }).click();
    await expect(page.getByText("已导出 1 份", { exact: true })).toBeVisible();
    await page.getByRole("checkbox", { name: "选择一班", exact: true }).check();
    await page.getByRole("button", { name: "导出 1 份", exact: true }).click();
    expect(await page.evaluate(() => (globalThis as any).__formExports)).toEqual([[], ["unit"]]);

    await page.getByRole("button", { name: "导入登记表", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "导入报名登记表", exact: true });
    await expect(dialog.getByText("这是「春季运动会」的表", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "导入", exact: true })).toBeDisabled();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    await dialog.getByRole("button", { name: "重新选择", exact: true }).click();
    await expect(dialog.getByText("这是「春季运动会」的表", { exact: true })).toBeHidden();
    await expect(dialog.getByText("队伍 1", { exact: true })).toBeVisible();
    await expect(dialog.getByText("接力 2", { exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "导入", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(await page.evaluate(() => (globalThis as any).__formImports)).toBe(1);

    await page.getByRole("button", { name: "粘贴表格", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "批量导入运动员", exact: true })).toBeVisible();
  });
}

test("报名页按组别导出，点行看运动员", async ({ page }) => {
  await page.setViewportSize({ width: 800, height: 800 });
  await installRuntime(page, { groups: true, limits: true });
  await page.goto("/");

  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "报名", exact: true }).click();
  await expect(page.getByText("超限 2", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

  await page.getByRole("radio", { name: "按组别", exact: true }).click();
  await page.getByRole("checkbox", { name: "选择未分组", exact: true }).check();
  await page.getByRole("button", { name: "导出 1 份", exact: true }).click();
  await page.getByRole("checkbox", { name: "选择未分组", exact: true }).uncheck();
  await page.getByRole("button", { name: "导出登记表", exact: true }).click();
  expect(await page.evaluate(() => (globalThis as any).__formExports)).toEqual(["group", "group"]);
  expect(await page.evaluate(() => (globalThis as any).__formExportIds)).toEqual([[""], []]);

  await page.getByRole("radio", { name: "按单位", exact: true }).click();
  await page.getByText("一班", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "运动员", exact: true })).toBeVisible();
});
