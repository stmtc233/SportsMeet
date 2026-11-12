import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

test("页面出错后可返回全部赛事", async ({ page }) => {
  await installRuntime(page, { brokenEvent: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await expect(page.getByText("页面出错，未保存的改动已丢失", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "返回全部赛事", exact: true }).click();
  await expect(page.getByText("秋季运动会", { exact: true })).toBeVisible();
  await expect(page.getByText("页面出错，未保存的改动已丢失", { exact: true })).toHaveCount(0);
});
