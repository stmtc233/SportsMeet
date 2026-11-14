import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`长名单滚动、筛选与窗口高度 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 500 });
    await installRuntime(page, { athleteCount: 1000 });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "报名", exact: true }).click();
    await page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^全部 \d/ }).click();
    const table = page.getByRole("table");
    await expect(page.getByText("运动员1", { exact: true })).toBeVisible();
    await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(page.getByText("运动员1000", { exact: true })).toBeVisible();
    const search = page.getByPlaceholder("搜索姓名、号码、学号");
    await search.fill("1"); // still above the virtualization threshold
    await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(page.getByText("运动员1000", { exact: true })).toBeVisible();
    await search.fill("运动员1000"); // below the threshold, scroll position clamps
    await expect(table.getByRole("row")).toHaveCount(2);
    await search.fill("");
    await expect(page.getByText("运动员1", { exact: true })).toBeVisible();
    await page.setViewportSize({ width, height: 1000 });
    await expect.poll(() => table.getByRole("row").count()).toBeGreaterThan(30);
    expect(await table.getByRole("row").count()).toBeLessThan(100);
    await table.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(page.getByText("运动员1000", { exact: true })).toBeVisible();
  });
}
