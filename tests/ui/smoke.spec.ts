import { expect, test, type Page } from "@playwright/test";
import { expectReport, installRuntime } from "./runtime";

async function noOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollHeight <= innerHeight)).toBe(true);
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
      await page.getByRole("button", { name: /^AI\/MCP/ }).click();
      await expect(page.getByRole("heading", { name: "AI/MCP", exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "支持的 Agent", exact: true })).toBeVisible();
      for (const name of ["TRAE", "Qoder", "CodeBuddy", "WorkBuddy", "Qwen Code", "Kimi Code", "Claude Code", "Codex", "Cursor", "Gemini CLI", "Google Antigravity", "Cline"]) await expect(page.getByText(name, { exact: true })).toBeVisible();
      await expect(page.locator(".mcp-agent svg[aria-hidden=true]")).toHaveCount(12);
      await expect(page.locator(".mcp-agent").last()).toHaveText("…");
      await noOverflow(page);
      await page.getByRole("button", { name: "返回", exact: true }).click();
      await page.getByRole("button", { name: "设置", exact: true }).click();
      const nav = page.getByRole("navigation", { name: "设置分类" });
      for (const section of ["通用", "操作日志", "关于"]) {
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
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await page.getByRole("button", { name: /^AI\/MCP/ }).click();
  const port = page.getByLabel("端口", { exact: true });
  await port.fill("18766");
  await page.setViewportSize({ width: 800, height: 800 });
  await expect(port).toBeFocused();
  await expect(port).toHaveValue("18766");
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "放弃未保存的改动？" })).toBeVisible();
  await page.getByRole("button", { name: "继续编辑", exact: true }).click();
  await expect(port).toHaveValue("18766");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
  await page.getByRole("switch", { name: "允许本机连接" }).click();
  await expect(page.getByText("已停止", { exact: true })).toBeVisible();
  await page.getByRole("switch", { name: "允许本机连接" }).click();
  await expect(page.getByText("运行中", { exact: true })).toBeVisible();
  await port.fill("9999");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("未能更新，请检查端口", { exact: true })).toBeVisible();
  await expect(port).toHaveValue("9999");
  await page.getByRole("button", { name: "放弃", exact: true }).click();
  await expect(port).toHaveValue("18766");
});

test("总览的「未分组」单位进报名并选中「未分组」", async ({ page }) => {
  await installRuntime(page, { groups: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.locator(".overview__phases").getByRole("button", { name: /^参赛单位.*未分组 1/ }).click();
  await expect(page.getByRole("heading", { name: "报名", exact: true })).toBeVisible();
  await expect(page.getByRole("complementary", { name: "组别与单位" }).getByRole("button", { name: /^未分组 \d/ })).toHaveAttribute("aria-current", "true");
});

for (const width of [1280, 800]) {
  test(`总览显示工作流与赛程 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { limits: true, conflicts: true });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
    await expect(page.getByRole("progressbar", { name: "报名 1 / 2", exact: true })).toBeVisible();
    // Three phases, the steps numbered on through them, the first not done marked.
    const phases = page.locator(".overview__phases");
    await expect(phases.getByRole("heading", { level: 2 })).toHaveText(["赛前", "比赛", "赛后"]);
    await expect(phases.locator(".overview-step__index")).toHaveText(["", "", "3", "", "5", "6", "7", "8"]);
    const current = phases.locator("[aria-current=step]");
    await expect(current).toHaveCount(1);
    await expect(current).toContainText("报名");
    await expect(phases.getByText("超限 2", { exact: true })).toBeVisible();
    await expect(phases.getByText("冲突 1", { exact: true })).toBeVisible();
    await expect(phases.getByText("待晋级 1", { exact: true })).toBeVisible();
    await expect(phases.getByText("待裁定 1", { exact: true })).toBeVisible();
    await expect(phases.getByText("未启用", { exact: true })).toBeVisible();
    // 团体总分 prints only once enabled.
    await expect(phases.getByRole("button", { name: "团体总分", exact: true })).toHaveCount(0);
    await noOverflow(page);
    await page.getByRole("button", { name: /^报名\s*1 \/ 2/ }).click();
    await expect(page.getByRole("heading", { name: "报名", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "总览", exact: true }).click();
    // A phase's print button opens 打印 on that report.
    await page.locator(".overview__phases").getByRole("button", { name: "检录单", exact: true }).click();
    await expectReport(page, "检录单");
    await page.getByRole("button", { name: "总览", exact: true }).click();
    // 赛程: an hour a row, now splitting its hour; a day of the meet switches.
    const chart = page.locator(".timeline");
    await expect(chart.locator(".timeline__hour")).toHaveText(["08:00", "09:00", "10:00"]);
    await expect(chart.locator(".timeline__now-text")).toHaveText("09:30");
    await expect(chart.getByRole("button", { name: /^男子跳远 · 08:30 · 未编排 .*已过开始时间/ })).toContainText("已过开始时间");
    await expect(chart.getByRole("button", { name: /^男子跳高 .*下一场$/ })).toContainText("下一场");
    if (width > 900) await expect(chart.locator(".timeline__lane-name")).toHaveText(["径赛", "田赛"]);
    else await expect(chart.locator(".timeline__row:not(.timeline__head)").first().locator(".timeline__cell-lane:visible")).toHaveText(["田赛"]);
    await page.getByRole("radio", { name: "11-13 周五" }).click();
    await expect(chart.getByRole("button", { name: /^跳绳 · 08:30/ })).toBeVisible();
    await page.getByRole("radio", { name: "11-12 周四" }).click();
    await expect(page.getByRole("button", { name: "1 个轮次未安排时间" })).toBeVisible();
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
    await page.locator(".timeline").getByRole("button", { name: /^男子100m · 09:00–09:40/ }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toHaveAttribute("aria-checked", "true");
    // The event stays under 总览 in the sidebar, and its back button goes there.
    await expect(page.getByRole("navigation").getByRole("button", { name: "总览", exact: true })).toHaveAttribute("aria-current", "true");
    await page.getByRole("radio", { name: "编排", exact: true }).click();
    await page.getByRole("button", { name: "返回总览", exact: true }).click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
  });
}

for (const width of [1280, 800]) {
  test(`赛程在一天排满时每轮写全名称 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page, { denseTimeline: true });
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    const chart = page.locator(".timeline");
    // Rounds minutes apart each get a line with their time and full name.
    await expect(chart.getByRole("button", { name: /^男子4×100m 初一组 · / })).toContainText("09:50男子4×100m 初一组");
    const clipped = await chart.locator(".timeline__name").evaluateAll((list) => list.filter((n) => n.scrollWidth > n.clientWidth).map((n) => n.textContent));
    expect(clipped).toEqual([]);
    await noOverflow(page);
  });
}

for (const width of [1280, 800]) {
  test(`AI/MCP 独立入口随开关高亮 ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await installRuntime(page);
    await page.goto("/");
    const entry = page.getByRole("button", { name: /^AI\/MCP/ });
    await expect(entry).toHaveClass(/st-button--tonal/);
    await expect(entry).toContainText("已开启");
    await entry.click();
    await expect(page.getByRole("switch", { name: "允许本机连接" })).toHaveAttribute("aria-checked", "true");
    await page.getByRole("switch", { name: "允许本机连接" }).click();
    await expect(page.getByText("已停止", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "返回", exact: true }).click();
    await expect(entry).toHaveClass(/st-button--secondary/);
    await expect(entry).not.toContainText("已开启");
    await page.getByText("秋季运动会", { exact: true }).click();
    await expect(entry).not.toHaveAttribute("data-highlighted");
    await entry.click();
    await page.getByRole("switch", { name: "允许本机连接" }).click();
    await expect(page.getByText("运行中", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "返回", exact: true }).click();
    await expect(page.getByRole("heading", { name: "总览", exact: true })).toBeVisible();
    await expect(entry).toHaveAttribute("data-highlighted", "true");
    await expect(entry).not.toHaveAttribute("aria-current");
    await page.getByRole("button", { name: "设置", exact: true }).click();
    await expect(page.getByRole("navigation", { name: "设置分类" }).getByRole("button", { name: /MCP/ })).toHaveCount(0);
  });
}
