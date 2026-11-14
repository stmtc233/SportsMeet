import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function noOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`裁定并列 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await installRuntime(page, { ties: true });
      await page.goto("/");
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "项目", exact: true }).click();
      await page.getByText("男子100m 初中组", { exact: true }).click();
      await page.getByRole("radio", { name: "成绩", exact: true }).click();

      const ranking = page.locator(".st-card").filter({ hasText: "预赛排名" });
      await expect(ranking.getByText("多 1 人晋级")).toBeVisible();
      await ranking.getByRole("button", { name: "裁定并列", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "裁定并列", exact: true });
      await expect(dialog).toBeVisible();
      await expect(dialog.locator(".tie-head")).toHaveText("第 3 名多 1 人晋级");
      await expect(dialog.getByLabel("运动员2的名次", { exact: true })).toHaveText("第 3 名");
      await noOverflow(page);
      const save = dialog.getByRole("button", { name: "保存", exact: true });
      await expect(save).toBeDisabled();

      await dialog.getByLabel("运动员2的名次", { exact: true }).click();
      await page.getByRole("option", { name: "第 4 名", exact: true }).click();
      await save.click();
      await expect(dialog).toHaveCount(0);
      expect(await page.evaluate(() => (globalThis as any).__tieCalls)).toEqual([
        { roundId: "round", places: [{ competitorId: "a1", place: 4 }, { competitorId: "a3", place: 3 }] },
      ]);
      await expect(ranking.getByText("多 1 人晋级")).toHaveCount(0);
      await expect(ranking.getByRole("row").last()).toContainText("运动员2");
      expect(errors).toEqual([]);
    });
  }
}
