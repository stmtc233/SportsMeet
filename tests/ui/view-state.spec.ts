import { expect, test } from "@playwright/test";
import { installRuntime, selectReport } from "./runtime";

test("切换页面后恢复滚动位置与筛选", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 600 });
  await installRuntime(page, { athleteCount: 1000 });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "运动员", exact: true }).click();
  const table = page.getByRole("table");
  await expect(page.getByText("运动员1", { exact: true })).toBeVisible();
  await page.getByPlaceholder("搜索姓名、号码、学号").fill("运动员");
  await table.evaluate((el) => { el.scrollTop = 4000; el.dispatchEvent(new Event("scroll")); });
  await expect(page.getByText("运动员101", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "项目", exact: true }).click();
  await expect(page.getByText("男子100m 初中组", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "运动员", exact: true }).click();
  await expect(page.getByPlaceholder("搜索姓名、号码、学号")).toHaveValue("运动员");
  await expect.poll(() => table.evaluate((el) => el.scrollTop)).toBe(4000);
  await expect(page.getByText("运动员101", { exact: true })).toBeVisible();
  // Opening the meet again returns to the page last shown.
  await page.getByRole("button", { name: "全部赛事", exact: true }).click();
  await page.getByText("秋季运动会", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "运动员", exact: true })).toBeVisible();
});

test("编排与成绩共用所选轮次", async ({ page }) => {
  await installRuntime(page, { eventConfig: "height", heightRounds: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子跳高", { exact: true }).click();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
  await page.getByRole("radio", { name: "半决赛", exact: true }).click();
  await page.getByRole("radio", { name: "成绩", exact: true }).click();
  await expect(page.getByRole("radio", { name: "半决赛", exact: true })).toHaveAttribute("aria-checked", "true");
  await page.getByRole("button", { name: "返回项目列表", exact: true }).click();
  await page.getByText("男子跳高", { exact: true }).click();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
  await expect(page.getByRole("radio", { name: "半决赛", exact: true })).toHaveAttribute("aria-checked", "true");
});

test("设置打开上次的分类，打印选择保留", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installRuntime(page);
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await expect(page.getByRole("heading", { name: "打印", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await selectReport(page, "检录单");
  await page.getByRole("button", { name: "项目", exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await expect(page.getByRole("button", { name: "报表种类", exact: true })).toHaveText("检录单");
  // Kept across a restart.
  await page.reload();
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await expect(page.getByRole("button", { name: "报表种类", exact: true })).toHaveText("检录单");
});
