import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`操作日志筛选与加载更多 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");
    await page.getByRole("button", { name: "设置", exact: true }).click();
    await page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: "操作日志", exact: true }).click();
    await expect(page.getByRole("heading", { name: "操作日志", exact: true })).toBeVisible();
    const rows = page.getByRole("table").getByRole("row");
    await expect(page.getByText("记录60", { exact: true })).toBeVisible();
    await expect(page.getByText("2026-11-12 10:59", { exact: true })).toBeVisible();
    await expect(rows).toHaveCount(51); // header + the first page
    const more = page.getByRole("button", { name: "加载更多", exact: true });
    await more.click();
    await expect(rows).toHaveCount(61);
    await expect(more).toBeDisabled();
    await page.getByRole("radio", { name: "MCP", exact: true }).click();
    await expect(rows).toHaveCount(21);
    await expect(page.getByText("记录59", { exact: true })).toHaveCount(0);
    await expect(page.getByText("记录60", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("成绩与编排显示 Go 整理的时间", async ({ page }) => {
  await installRuntime(page, { schedule: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await expect(page.getByRole("table").getByText("09:00", { exact: true })).toBeVisible();
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await page.getByRole("radio", { name: "成绩", exact: true }).click();
  await expect(page.getByText("修改于 2026-11-12 09:20", { exact: true })).toBeVisible();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
  await page.getByRole("button", { name: "更多", exact: true }).nth(1).click();
  await page.getByRole("menuitem", { name: "编辑组…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "编辑组", exact: true });
  await expect(dialog.getByLabel("计划日期")).toHaveValue("2026-11-12");
  await dialog.getByLabel("时间", { exact: true }).fill("10:15");
  await dialog.getByLabel("预计用时").fill("8");
  await dialog.getByRole("button", { name: "确定", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.locator(".st-tag", { hasText: "10:15" })).toBeVisible();
  await expect(page.getByText("8 分钟", { exact: true })).toBeVisible();
  const save = page.getByRole("button", { name: "保存", exact: true });
  await save.click();
  const sent = await page.evaluate(() => (globalThis as any).__relayCalls.find((c: any) => c.method === "Arrange.Save").args[0].rounds[0].changed);
  expect(sent.map((h: any) => [h.scheduledAt, h.durationMinutes])).toEqual([["2026-11-12T10:15", 8]]);
});
