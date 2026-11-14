import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function noOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`核心页面 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await installRuntime(page);
      await page.goto("/");
      await expect(page.getByText("秋季运动会", { exact: true })).toBeVisible();
      await noOverflow(page);
      await page.getByRole("button", { name: "新建赛事", exact: true }).click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await page.getByLabel("赛事名称").fill("测试赛事");
      await noOverflow(page);
      await page.getByRole("button", { name: "取消", exact: true }).click();
      await page.getByText("秋季运动会", { exact: true }).click();
      await page.getByRole("button", { name: "项目", exact: true }).click();
      await expect(page.getByRole("heading", { name: "项目", exact: true })).toBeVisible();
      await noOverflow(page);
      for (const label of ["赛事设置", "报名", "日程", "团体总分", "打印"]) {
        await page.getByRole("navigation").getByRole("button", { name: label, exact: true }).click();
        await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
        await noOverflow(page);
      }
      await page.getByRole("navigation").getByRole("button", { name: "报名", exact: true }).click();
      // The tree's unit lists its athletes; a narrow window shows one side at a time.
      const tree = page.getByRole("complementary", { name: "组别与单位" });
      await tree.getByRole("button", { name: /^一班 .*\d/ }).click();
      await expect(page.getByRole("heading", { name: "一班 24 人", exact: true })).toBeVisible();
      await expect(tree).toBeVisible({ visible: width > 900 });
      await noOverflow(page);
      if (width === 800) await page.getByRole("button", { name: "组别与单位", exact: true }).click();
      await tree.getByRole("button", { name: /^全部 \d/ }).click();
      await expect(page.getByRole("heading", { name: "全部 24 人", exact: true })).toBeVisible();
      await page.getByRole("button", { name: "项目", exact: true }).click();
      await page.getByText("男子100m 初中组", { exact: true }).click();
      await noOverflow(page);
      for (const label of ["编排", "成绩", "报名"]) {
        await page.getByRole("radio", { name: label, exact: true }).click();
        await noOverflow(page);
      }
      await page.getByRole("button", { name: "设置", exact: true }).click();
      const nav = page.getByRole("navigation", { name: "设置分类" });
      for (const section of ["通用", "编排", "打印", "MCP", "操作日志", "关于"]) {
        await nav.getByRole("button", { name: section, exact: true }).click();
        await expect(page.getByRole("heading", { name: section, exact: true })).toBeVisible();
        await noOverflow(page);
        if (width === 800) await page.getByRole("button", { name: "设置分类", exact: true }).click();
      }
      expect(errors).toEqual([]);
    });
  }
}

test("设置保存、宽窄切换与离开确认", async ({ page }) => {
  await installRuntime(page);
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.getByRole("button", { name: "通用", exact: true }).click();
  await page.getByRole("radio", { name: "深色", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "MCP", exact: true }).click();
  const port = page.getByLabel("端口", { exact: true });
  await port.fill("18766");
  await page.setViewportSize({ width: 800, height: 800 });
  await expect(port).toBeFocused();
  await expect(port).toHaveValue("18766");
  await page.getByRole("button", { name: "设置分类", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "放弃未保存的改动？" })).toBeVisible();
  await page.getByRole("button", { name: "继续编辑", exact: true }).click();
  await expect(port).toHaveValue("18766");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
  await page.getByRole("switch", { name: "允许本机连接" }).click();
  await expect(page.getByText("运行中", { exact: true })).toBeVisible();
  await port.fill("9999");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("未能更新，请检查端口", { exact: true })).toBeVisible();
  await expect(port).toHaveValue("9999");
  await page.getByRole("button", { name: "放弃", exact: true }).click();
  await expect(port).toHaveValue("18766");
});

for (const width of [1280, 800]) {
  test(`总览显示工作流与待比赛 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { limits: true, conflicts: true });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
    await expect(page.getByRole("progressbar", { name: "报名 1 / 2", exact: true })).toBeVisible();
    const steps = page.locator(".overview__steps");
    await expect(steps.getByText("超限 2", { exact: true })).toBeVisible();
    await expect(steps.getByText("冲突 1", { exact: true })).toBeVisible();
    await noOverflow(page);
    await page.getByRole("button", { name: /^报名\s*1 \/ 2/ }).click();
    await expect(page.getByRole("heading", { name: "报名", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "总览", exact: true }).click();
    // 赛事结构: a unit opens selected in the 报名 tree, an event on its own page.
    const map = page.locator(".mindmap");
    await expect(map.getByRole("button", { name: "二班 0 未报名", exact: true })).toBeVisible();
    await map.getByRole("button", { name: "一班 24 超限 2", exact: true }).click();
    await expect(page.getByRole("heading", { name: "一班 24 人", exact: true })).toBeVisible();
    if (width > 900) await expect(page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^一班 .*\d/ })).toHaveAttribute("aria-current", "true");
    await page.getByRole("button", { name: "总览", exact: true }).click();
    await page.locator(".mindmap").getByRole("button", { name: "男子100m 进行中", exact: true }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toHaveAttribute("aria-checked", "true");
    await page.getByRole("button", { name: "返回总览", exact: true }).click();
    await page.getByText("男子100m 决赛", { exact: true }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toHaveAttribute("aria-checked", "true");
    // The event stays under 总览 in the sidebar, and its back button goes there.
    await expect(page.getByRole("navigation").getByRole("button", { name: "总览", exact: true })).toHaveAttribute("aria-current", "true");
    await page.getByRole("radio", { name: "编排", exact: true }).click();
    await page.getByRole("button", { name: "返回总览", exact: true }).click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
  });
}
