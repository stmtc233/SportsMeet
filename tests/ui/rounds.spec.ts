import { expect, test } from "@playwright/test";
import { expectReport, installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`编排页批量自动编排 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("navigation").getByRole("button", { name: "编排", exact: true }).click();
    await expect(page.getByRole("heading", { name: "编排", exact: true })).toBeVisible();
    // Opens on what can be arranged now.
    await expect(page.getByRole("button", { name: /女子跳远/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /^男子100m/ })).toHaveCount(0);
    await page.getByRole("radio", { name: "全部", exact: true }).click();
    await expect(page.getByText("等决赛")).toHaveCount(0);
    await expect(page.getByText("未排 6")).toHaveCount(0);
    const auto = page.getByRole("button", { name: /^自动编排/ });
    await expect(auto).toBeDisabled();
    await page.getByRole("checkbox", { name: "选择女子跳远", exact: true }).check();
    await page.getByRole("checkbox", { name: "选择男子100m", exact: true }).check();
    // Only rounds that can be arranged count.
    await expect(auto).toHaveAccessibleName("自动编排 1 个");
    await auto.click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: "自动编排 1 个轮次" })).toBeVisible();
    await dialog.getByRole("button", { name: "预览", exact: true }).click();
    await expect(dialog.getByText("1 组 · 6 人", { exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "编排 1 个轮次", exact: true }).click();
    await expect(dialog).toBeHidden();
    const saved = await page.evaluate(() => (globalThis as any).__batchSaved);
    expect(saved[1]).toBe("batch");
    expect(saved[0].rounds).toEqual([{ eventId: "jump", roundId: "jump-final" }]);
    // A row opens its event's 编排 and comes back here.
    await page.getByRole("button", { name: "男子100m", exact: true }).click();
    await expect(page.getByRole("radio", { name: "编排", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "返回编排", exact: true }).click();
    await expect(page.getByRole("heading", { name: "编排", exact: true })).toBeVisible();
  });

  test(`成绩页按日期列出轮次 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("navigation").getByRole("button", { name: "成绩", exact: true }).click();
    await expect(page.getByRole("heading", { name: "成绩", exact: true })).toBeVisible();
    await expect(page.getByRole("radio", { name: "2026-11-12 周四", exact: true })).toBeChecked();
    await expect(page.getByText("录入中 1/3", { exact: true })).toBeVisible();
    await expect(page.getByText("可编排", { exact: true })).toBeVisible();
    await page.getByRole("radio", { name: "2026-11-13 周五", exact: true }).click();
    await expect(page.getByText("已录完", { exact: true })).toBeVisible();
    await page.getByRole("radio", { name: "2026-11-12 周四", exact: true }).click();
    // Each round prints its own 检录单.
    await page.getByRole("button", { name: "打印男子100m", exact: true }).click();
    await page.getByRole("menuitem", { name: "检录单", exact: true }).click();
    await expect(page.getByRole("heading", { name: "打印", exact: true })).toBeVisible();
    await expectReport(page, "检录单");
    await page.getByRole("navigation").getByRole("button", { name: "成绩", exact: true }).click();
    // A round opens its event's 成绩, and 「下一场」 goes on in time.
    await page.getByRole("button", { name: "男子100m", exact: true }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "下一场", exact: true }).click();
    await expect(page.getByRole("radio", { name: "编排", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "返回成绩", exact: true }).click();
    await expect(page.getByRole("heading", { name: "成绩", exact: true })).toBeVisible();
  });
}
