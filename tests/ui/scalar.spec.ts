import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`计次零值与多次成绩 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { eventConfig: "count" });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "项目", exact: true }).click();
    await page.getByText("男子跳绳", { exact: true }).click();
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await page.getByRole("textbox", { name: "尝试 1", exact: true }).nth(0).fill("0");
    for (const [i, v] of ["120", "124", "122"].entries()) await page.getByRole("textbox", { name: `尝试 ${i + 1}`, exact: true }).nth(1).fill(v);
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await expect(page.locator(".result-mark").nth(0)).toHaveText("0");
    await expect(page.locator(".result-mark").nth(1)).toHaveText("124");
    await page.screenshot({ path: `test-results/count-${width}.png`, animations: "disabled" });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
  test(`计分固定精度 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { eventConfig: "score" });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "项目", exact: true }).click();
    await page.getByText("男子技巧评分", { exact: true }).click();
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await page.getByRole("textbox", { name: "分数", exact: true }).nth(0).fill("12.5");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await expect(page.getByRole("textbox", { name: "分数", exact: true }).nth(0)).toHaveValue("12.50");
  });
  test(`普通队伍计次 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { eventConfig: "teamCount" });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "项目", exact: true }).click();
    await page.getByText("男子集体跳绳", { exact: true }).click();
    await page.getByRole("button", { name: "管理队伍", exact: true }).first().click();
    await page.getByRole("button", { name: "添加队伍", exact: true }).click();
    const editor = page.getByRole("dialog", { name: "添加队伍", exact: true });
    await editor.getByLabel("队名", { exact: true }).fill("一班队");
    await expect(editor.getByText("人数与项目建议不符", { exact: true })).toBeVisible();
    for (const i of [1, 2]) {
      await editor.getByLabel("添加成员", { exact: true }).click();
      await page.getByRole("option", { name: `运动员${i} · ${99 + i}`, exact: true }).click();
    }
    await expect(editor.getByText("人数与项目建议不符", { exact: true })).toHaveCount(0);
    await editor.getByRole("button", { name: "保存", exact: true }).click();
    const teams = page.getByRole("dialog", { name: "队伍", exact: true });
    await teams.getByRole("checkbox", { name: "报名", exact: true }).check();
    await teams.getByRole("button", { name: "报名 1 队", exact: true }).click();
    await page.getByRole("radio", { name: "编排", exact: true }).click();
    await page.getByRole("button", { name: "自动编排", exact: true }).first().click();
    const auto = page.getByRole("dialog", { name: "自动编排", exact: true });
    await expect(auto.getByLabel("每组队数", { exact: true })).toHaveValue("12");
    await auto.getByRole("button", { name: "生成", exact: true }).click();
    await expect(page.locator(".heat-slots .chip__name")).toHaveText(["一班队"]);
    await page.locator(".subbar").getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.locator(".subbar").getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await page.getByRole("textbox", { name: "尝试 1", exact: true }).fill("125");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await expect(page.getByText("125", { exact: true })).toBeVisible();
  });
}
