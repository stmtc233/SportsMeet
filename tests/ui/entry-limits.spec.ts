import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`报名限制只提示不拦截 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { limits: true });
    await page.goto("/");

    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "编辑赛事", exact: true }).click();
    const meet = page.getByRole("dialog", { name: "编辑赛事", exact: true });
    await expect(meet.getByLabel("每人限报项数", { exact: true })).toHaveValue("2");
    await meet.getByLabel("单位每项限报人数", { exact: true }).fill("3人");
    await expect(meet.getByLabel("单位每项限报人数", { exact: true })).toHaveValue("3");
    await meet.getByLabel("每人限报项数", { exact: true }).fill("");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await meet.getByRole("button", { name: "保存", exact: true }).click();
    const saved = await page.evaluate(() => (globalThis as any).__meetCalls[0][1].entryLimits);
    expect(saved).toEqual({ athleteEvents: 0, unitAthletes: 3, unitTeams: 0 });

    await expect(meet).toBeHidden();
    await page.getByText("男子100m 初中组", { exact: true }).click();
    await page.getByRole("radio", { name: "报名", exact: true }).click();
    await expect(page.getByText("超出限报项数", { exact: true })).toHaveCount(1);
    await page.getByRole("button", { name: "从赛事添加", exact: true }).click();
    const add = page.getByRole("dialog", { name: "报名「男子100m 初中组」", exact: true });
    await add.getByRole("checkbox", { name: "选择运动员3", exact: true }).check();
    await expect(add.getByText("「运动员3」报 3 项，限报 2 项", { exact: true })).toBeVisible();
    await add.getByRole("button", { name: "报名 1 人", exact: true }).click();
    await expect(page.getByText("3 人报名", { exact: true })).toBeVisible();
  });
}
