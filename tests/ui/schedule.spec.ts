import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function openArrange(page: Page, options = {}) {
  await installRuntime(page, { schedule: true, ...options });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
}
async function openSchedule(page: Page) {
  await page.locator(".subbar").getByRole("button", { name: "更多", exact: true }).click();
  await page.getByRole("menuitem", { name: "批量排时间…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "批量排时间", exact: true });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("首组日期").fill("2026-11-12");
  await dialog.getByLabel("首组时间").fill("23:58");
  await dialog.getByLabel("间隔", { exact: true }).fill("3");
  return dialog;
}

for (const width of [1280, 800]) {
  test(`批量排时预览、保存和刷新 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await openArrange(page);
    const dialog = await openSchedule(page);
    await dialog.getByRole("button", { name: "预览", exact: true }).click();
    await expect(dialog.getByRole("heading", { name: "修改 2 组", exact: true })).toBeVisible();
    await expect(dialog.getByText("2026-11-13 00:01", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "跳过 1 组", exact: true })).toBeVisible();
    await expect(dialog.getByText("已有成绩", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (globalThis as any).__scheduleCalls.save)).toBe(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    // Editing any parameter invalidates the confirmation and requires a new preview.
    await dialog.getByLabel("间隔", { exact: true }).fill("4");
    await expect(dialog.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
    await dialog.getByLabel("间隔", { exact: true }).fill("3");
    await dialog.getByRole("button", { name: "预览", exact: true }).click();
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.locator(".heat-slots")).toHaveCount(3);
    await expect(page.locator(".split__main").getByText("23:58", { exact: true })).toBeVisible();
    await expect(page.locator(".split__main").getByText("00:01", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (globalThis as any).__scheduleCalls.save)).toBe(1);
  });
}

test("编排草稿阻止批量排时", async ({ page }) => {
  await openArrange(page);
  await page.getByRole("button", { name: "添加组", exact: true }).click();
  await page.locator(".subbar").getByRole("button", { name: "更多", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: "批量排时间…", exact: true })).toBeDisabled();
  expect(await page.evaluate(() => (globalThis as any).__scheduleCalls.preview)).toBe(0);
});

test("过期排时提交须重新预览", async ({ page }) => {
  await openArrange(page, { staleSchedule: true });
  const dialog = await openSchedule(page);
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog.getByText("未能保存，请重新预览", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "预览", exact: true })).toBeVisible();
  await expect(page.locator(".split__main").getByText("23:58", { exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toHaveCount(0);
});

test("外部轮次变化使已展示预览失效", async ({ page }) => {
  await openArrange(page);
  const dialog = await openSchedule(page);
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "保存", exact: true })).toBeVisible();
  await page.evaluate(() => (globalThis as any).__scheduleExternalChange());
  await expect(dialog.getByText("数据已变化，请重新预览", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
});

test("排时校验错误无法保存", async ({ page }) => {
  await openArrange(page, { scheduleErrors: true });
  const dialog = await openSchedule(page);
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await expect(dialog.getByText("间隔须为 1–1440 分钟", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
});
