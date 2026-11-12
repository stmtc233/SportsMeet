import { expect, test } from "@playwright/test";
import { installRuntime } from "./runtime";

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`项目预设与配置 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      await installRuntime(page);
      await page.goto("/");
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "添加项目", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "添加项目", exact: true });
      await dialog.getByLabel("常用项目", { exact: true }).click();
      await page.getByRole("menuitemradio", { name: "中长跑 · 1500m", exact: true }).click();
      await expect(dialog.getByLabel("项目名", { exact: true })).toHaveValue("1500m");
      await expect(dialog.getByRole("radio", { name: "不分道", exact: true })).toBeChecked();
      await expect(dialog.getByLabel("道数", { exact: true })).toBeDisabled();
      await dialog.getByLabel("常用项目", { exact: true }).click();
      await page.getByRole("menuitemradio", { name: "投掷 · 铅球", exact: true }).click();
      await expect(dialog.getByLabel("试投次数", { exact: true })).toHaveValue("3");
      await dialog.getByLabel("项目名", { exact: true }).fill("自定义投掷");
      await expect(dialog.getByRole("radio", { name: "投掷", exact: true })).toBeChecked();
      await dialog.getByLabel("常用项目", { exact: true }).click();
      await page.getByRole("menuitemradio", { name: "接力 · 混合4×400m", exact: true }).click();
      await expect(dialog.getByRole("radio", { name: "混合", exact: true })).toBeChecked();
      await expect(dialog.getByRole("radio", { name: "分道后抢道", exact: true })).toBeChecked();
      await expect(dialog.getByLabel("建议棒数", { exact: true })).toHaveValue("4");
      await dialog.getByLabel("建议棒数", { exact: true }).fill("5");
      await page.screenshot({ path: `test-results/event-presets-${width}-${colorScheme}.png`, animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await dialog.getByRole("button", { name: "保存", exact: true }).click();
      await expect(dialog).not.toBeVisible();
      const calls = await page.evaluate(() => (globalThis as any).__eventCalls);
      expect(calls[0][2]).toMatchObject({ discipline: "4×400m", gender: "混合", kind: "relay", startMode: "break", relayLegCount: 5 });
      await page.getByRole("button", { name: "编辑", exact: true }).click();
      const edit = page.getByRole("dialog", { name: "编辑项目", exact: true });
      await expect(edit.getByLabel("建议棒数", { exact: true })).toHaveValue("5");
      await expect(edit.getByRole("radio", { name: "分道后抢道", exact: true })).toBeChecked();
      await expect(edit.getByLabel("常用项目", { exact: true })).toHaveCount(0);
    });
  }

  test(`不分道编排与计时 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { eventConfig: "group" });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByText("男子1500m", { exact: true }).click();
    await page.getByRole("radio", { name: "编排", exact: true }).click();
    await expect(page.locator(".heat-slots .chip__name")).toHaveCount(16);
    await expect(page.locator(".heat-slots .slot")).toHaveCount(17);
    await page.getByRole("button", { name: "自动编排", exact: true }).click();
    const auto = page.getByRole("dialog", { name: "自动编排", exact: true });
    await expect(auto.getByLabel("每组人数", { exact: true })).toHaveValue("12");
    await expect(auto.getByRole("radio", { name: "好成绩排中间道", exact: true })).toHaveCount(0);
    await auto.getByRole("button", { name: "取消", exact: true }).click();
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await expect(page.locator(".result-grid .head").first()).toHaveText("顺序");
    await expect(page.getByRole("textbox", { name: "成绩", exact: true })).toHaveCount(16);
    await page.getByRole("textbox", { name: "成绩", exact: true }).nth(15).fill("5:12.34");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    const calls = await page.evaluate(() => (globalThis as any).__relayCalls);
    expect(calls.find((c: any) => c.method === "Results.Save").args[1][0]).toMatchObject({ position: 16, mark: "5:12.34" });
  });

  test(`投掷逐次录入 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { eventConfig: "throw" });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByText("男子铅球", { exact: true }).click();
    await page.getByRole("radio", { name: "成绩", exact: true }).click();
    await expect(page.getByRole("textbox", { name: "试投 1", exact: true })).toHaveCount(8);
    await page.getByRole("textbox", { name: "试投 1", exact: true }).first().fill("8.50");
    await page.getByRole("textbox", { name: "试投 2", exact: true }).first().fill("X");
    await page.getByRole("textbox", { name: "试投 3", exact: true }).first().fill("免投");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
    const calls = await page.evaluate(() => (globalThis as any).__relayCalls);
    expect(calls.find((c: any) => c.method === "Results.Save").args[1][0].attempts).toEqual(["8.50", "X", "免投"]);
    await page.screenshot({ path: `test-results/throwing-${width}.png`, animations: "disabled" });
  });
}

test("外部起跑方式变化清除不适用的编排草稿", async ({ page }) => {
  await installRuntime(page, { emptyBoard: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByText("男子100m 初中组", { exact: true }).click();
  await page.getByRole("radio", { name: "编排", exact: true }).click();
  await page.locator(".drop-roster .chip").first().click();
  await page.locator(".heat-slots .slot").nth(3).click();
  await expect(page.locator(".heat-slots .chip__name")).toHaveCount(1);
  await expect(page.locator(".subbar").getByRole("button", { name: "保存", exact: true })).toBeEnabled();
  await page.evaluate(() => (globalThis as any).__changeToGroupStart());
  await expect(page.getByText("项目配置已修改", { exact: true })).toBeVisible();
  await expect(page.locator(".heat-slots .chip__name")).toHaveCount(0);
  await expect(page.locator(".heat-slots .slot")).toHaveCount(1);
  await expect(page.locator(".subbar").getByRole("button", { name: "保存", exact: true })).toBeDisabled();
});
