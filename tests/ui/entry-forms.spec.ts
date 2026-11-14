import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`报名页导出与导入登记表 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");

    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "报名", exact: true }).click();
    await expect(page.getByText("1 / 2 单位已报名", { exact: true })).toBeVisible();
    await expect(page.getByText("未报名", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    // 全部 exports every unit; a unit picked in the tree just its own.
    await page.getByRole("button", { name: "导出登记表", exact: true }).click();
    await expect(page.getByText("已导出 1 份", { exact: true })).toBeVisible();
    await page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^二班 .*\d/ }).click();
    await page.getByRole("button", { name: "导出登记表", exact: true }).click();
    expect(await page.evaluate(() => (globalThis as any).__formExports)).toEqual([[], ["unit2"]]);

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

    await page.getByRole("button", { name: "导入名单", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "导入名单", exact: true })).toBeVisible();
  });
}

test("报名页按组别导出，树上选组别与单位", async ({ page }) => {
  await page.setViewportSize({ width: 800, height: 800 });
  await installRuntime(page, { groups: true, limits: true });
  await page.goto("/");

  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "报名", exact: true }).click();
  const tree = page.getByRole("complementary", { name: "组别与单位" });
  await expect(tree.getByText("超限 2", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

  // 全部 offers a form per unit or per group; 未分组 its own units or one form.
  await page.getByRole("button", { name: "导出登记表", exact: true }).click();
  await page.getByRole("menuitem", { name: "每个组别一份（2 份）", exact: true }).click();
  await tree.getByRole("button", { name: /^未分组 \d/ }).click();
  await expect(page.locator(".roster-main__title")).toHaveText(/未分组\s*0 人/);
  await page.getByRole("button", { name: "导出登记表", exact: true }).click();
  await page.getByRole("menuitem", { name: "每个单位一份（1 份）", exact: true }).click();
  await page.getByRole("button", { name: "导出登记表", exact: true }).click();
  await page.getByRole("menuitem", { name: "「未分组」一份", exact: true }).click();
  expect(await page.evaluate(() => (globalThis as any).__formExports)).toEqual(["group", ["unit2"], "group"]);
  expect(await page.evaluate(() => (globalThis as any).__formExportIds)).toEqual([[], ["unit2"], [""]]);

  // A narrow window goes back to the tree; a unit lists its athletes.
  await page.getByRole("button", { name: "组别与单位", exact: true }).click();
  await tree.getByRole("button", { name: /^一班 .*\d/ }).click();
  await expect(page.locator(".roster-main__title")).toHaveText(/高一.*一班\s*24 人/);
  await expect(page.getByPlaceholder("搜索姓名、号码、学号")).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "参赛单位", exact: true })).toHaveCount(0);

  // A unit added with a group picked starts in it.
  await page.getByRole("button", { name: "组别与单位", exact: true }).click();
  await tree.getByRole("button", { name: /^高一 \d/ }).click();
  await page.getByRole("button", { name: "组别与单位", exact: true }).click();
  await tree.getByRole("button", { name: "添加单位", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "添加单位", exact: true }).getByRole("checkbox", { name: "高一", exact: true })).toBeChecked();
});
