import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) for (const theme of ["light", "dark"] as const) {
  test(`沿用项目与赛制，失败保留输入 ${width} ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await page.emulateMedia({ colorScheme: theme });
    await installRuntime(page, { rejectMeetCreate: true });
    await page.goto("/");
    await page.getByRole("button", { name: "新建赛事", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "新建赛事", exact: true });
    await dialog.getByLabel("赛事名称", { exact: true }).fill("春季运动会");
    await dialog.getByLabel("开始日期", { exact: true }).fill("2027-04-20");
    const source = dialog.getByLabel("沿用项目与赛制", { exact: true });
    await expect(source).toHaveText("不沿用");
    await source.focus();
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("End");
    await page.keyboard.press("Enter");
    await expect(source).toContainText("秋季运动会");
    await expect(source).toBeFocused();
    await dialog.getByLabel("每人限报项数", { exact: true }).fill("3");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByText("来源赛事不存在或已删除", { exact: true })).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("赛事名称", { exact: true })).toHaveValue("春季运动会");
    await expect(source).toContainText("秋季运动会");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: `test-results/meet-template-${width}-${theme}.png` });
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const calls = await page.evaluate(() => (globalThis as any).__meetCreateCalls);
    expect(calls).toHaveLength(2);
    expect(calls[1]).toMatchObject({ sourceMeetId: "meet", name: "春季运动会", startDate: "2027-04-20", entryLimits: { athleteEvents: 3 } });
    await page.getByRole("button", { name: "全部赛事", exact: true }).click();
    await expect(page.getByText("春季运动会", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "新建赛事", exact: true }).click();
    await expect(dialog.getByLabel("沿用项目与赛制", { exact: true })).toHaveText("不沿用");
    await expect(dialog.getByLabel("赛事名称", { exact: true })).toHaveValue("");
    await dialog.getByRole("button", { name: "取消", exact: true }).click();
    await page.getByRole("button", { name: "编辑", exact: true }).first().click();
    await expect(page.getByRole("dialog", { name: "编辑赛事", exact: true }).getByLabel("沿用项目与赛制", { exact: true })).toHaveCount(0);
  });
}

test("新建赛事默认不沿用，取消选择后可建空赛事", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByRole("button", { name: "新建赛事", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新建赛事", exact: true });
  const source = dialog.getByLabel("沿用项目与赛制", { exact: true });
  await source.click();
  await dialog.getByRole("option", { name: "秋季运动会 · 2026-11-12", exact: true }).click();
  await source.click();
  await dialog.getByRole("option", { name: "不沿用", exact: true }).click();
  await dialog.getByLabel("赛事名称", { exact: true }).fill("空赛事");
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => (globalThis as any).__meetCreateCalls[0].sourceMeetId)).toBe("");
});
