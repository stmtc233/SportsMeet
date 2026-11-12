import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  test(`接力队伍、棒次、报名、编排与成绩 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { relay: true });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByText("男子4×100m", { exact: true }).click();
    await page.getByRole("radio", { name: "报名", exact: true }).click();
    await page.getByRole("button", { name: "管理队伍", exact: true }).first().click();
    await page.getByRole("button", { name: "添加队伍", exact: true }).click();
    const editor = page.getByRole("dialog", { name: "添加队伍", exact: true });
    await editor.getByLabel("队名", { exact: true }).fill("一班A队");
    for (const i of [1, 2, 3, 4]) {
      await editor.getByLabel("添加下一棒", { exact: true }).click();
      await page.getByRole("menuitemradio", { name: `运动员${i} · ${99 + i}`, exact: true }).click();
      await expect(editor.getByLabel(`第 ${i} 棒`, { exact: true })).toHaveText(new RegExp(`运动员${i}`));
    }
    await editor.getByRole("button", { name: "上移第 2 棒", exact: true }).click();
    await expect(editor.getByLabel("第 1 棒", { exact: true })).toHaveText(/运动员2/);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await editor.getByRole("button", { name: "保存", exact: true }).click();
    const teams = page.getByRole("dialog", { name: "接力队伍", exact: true });
    await expect(teams.getByText("运动员2 → 运动员1 → 运动员3 → 运动员4", { exact: true })).toBeVisible();
    await teams.getByRole("checkbox", { name: "报名", exact: true }).check();
    await teams.getByRole("button", { name: "报名 1 队", exact: true }).click();
    await expect(page.getByText("1 队报名", { exact: true })).toBeVisible();
    await page.getByRole("radio", { name: "编排", exact: true }).click();
    await page.getByRole("button", { name: "自动编排", exact: true }).first().click();
    const auto = page.getByRole("dialog", { name: "自动编排", exact: true });
    await expect(auto.getByLabel("每组队数", { exact: true })).toHaveValue("8");
    await auto.getByRole("button", { name: "生成", exact: true }).click();
    await expect(page.locator(".heat-slots .chip__name")).toHaveText(["一班A队"]);
    await page.locator(".subbar").getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.locator(".subbar").getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await expect(page.locator(".heat-slots")).toHaveCount(1);
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await page.getByRole("textbox", { name: "成绩", exact: true }).fill("50.12");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    await expect(page.getByRole("textbox", { name: "成绩", exact: true })).toHaveValue("50.12");
    const calls = await page.evaluate(() => (globalThis as any).__relayCalls);
    expect(calls.find((c: any) => c.method === "Events.SaveTeam").args[2].memberIds).toEqual(["a1", "a0", "a2", "a3"]);
    expect(calls.find((c: any) => c.method === "Entries.Add").args[1]).toEqual([{ teamId: "team0" }]);
    expect(calls.find((c: any) => c.method === "Results.Save").args[1][0]).toMatchObject({ position: 4, mark: "50.12" });
    await page.getByRole("radio", { name: "报名", exact: true }).click();
    await page.getByRole("button", { name: "管理队伍", exact: true }).click();
    await page.screenshot({ path: `test-results/relay-teams-${width}.png`, animations: "disabled" });
    await teams.getByRole("button", { name: "编辑", exact: true }).click();
    const locked = page.getByRole("dialog", { name: "编辑队伍", exact: true });
    await expect(locked.getByLabel("第 1 棒", { exact: true })).toBeDisabled();
    await expect(locked.getByLabel("添加下一棒", { exact: true })).toBeDisabled();
    await page.screenshot({ path: `test-results/relay-${width}.png`, animations: "disabled" });
  });
}

test("接力保存失败保留队名和棒次", async ({ page }) => {
  await installRuntime(page, { relay: true, rejectTeamSave: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子4×100m", { exact: true }).click();
    await page.getByRole("radio", { name: "报名", exact: true }).click();
  await page.getByRole("button", { name: "管理队伍", exact: true }).first().click();
  await page.getByRole("button", { name: "添加队伍", exact: true }).click();
  const editor = page.getByRole("dialog", { name: "添加队伍", exact: true });
  await editor.getByLabel("队名", { exact: true }).fill("未保存的队伍");
  await editor.getByLabel("添加下一棒", { exact: true }).click();
  await page.getByRole("menuitemradio", { name: "运动员1 · 100", exact: true }).click();
  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("队伍已有比赛记录，不能改棒次", { exact: true })).toBeVisible();
  await expect(editor.getByLabel("队名", { exact: true })).toHaveValue("未保存的队伍");
  await expect(editor.getByLabel("第 1 棒", { exact: true })).toHaveText(/运动员1/);
});
