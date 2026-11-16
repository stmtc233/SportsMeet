import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`创建示例赛事与失败重试 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { emptyMeets: true, rejectExampleCreate: true });
    await page.goto("/");
    await expect(page.getByText("还没有赛事", { exact: true })).toBeVisible();
    const create = page.getByRole("button", { name: "创建示例赛事", exact: true });
    await create.click();
    await expect(create).toBeDisabled();
    await expect(create).toHaveAttribute("aria-busy", "true");
    await expect(page.getByText("示例赛事创建失败，请重试", { exact: true })).toBeVisible();
    await expect(create).toBeEnabled();
    await create.click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
    await expect(page.getByText("青禾市校园田径运动会（示例）", { exact: true }).first()).toBeVisible();
    await page.getByRole("button", { name: "全部赛事", exact: true }).click();
    await expect(page.getByRole("heading", { name: "赛事", exact: true })).toBeVisible();
    await create.click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
    expect(await page.evaluate(() => (globalThis as any).__exampleCreateCalls)).toBe(3);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}
