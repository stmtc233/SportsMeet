import { expect, test, type Page } from "@playwright/test";
import { installRuntime, selectReport } from "./runtime";

async function noOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

async function openMeet(page: Page) {
  await installRuntime(page, { schedule: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
}

const calls = (page: Page, name: string) => page.evaluate((name) => (globalThis as any)[name] ?? [], name);

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`实际开赛时间 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await openMeet(page);
      await page.getByText("男子100m 初中组", { exact: true }).click();
      await page.getByRole("radio", { name: "成绩", exact: true }).click();
      const first = page.locator(".st-card").filter({ hasText: "第1组" }).first();
      const second = page.locator(".st-card").filter({ hasText: "第2组" }).first();
      // A heat that has competed shows its start against the plan.
      await expect(second.locator("[data-started]")).toHaveText(/开赛 09:19\s*晚 12 分钟/);
      await expect(first.locator("[data-started]")).toHaveCount(0);

      // The first results record it.
      await first.getByRole("textbox", { name: "成绩", exact: true }).first().fill("12.34");
      await first.getByRole("button", { name: "保存", exact: true }).click();
      await expect(first.locator("[data-started]")).toHaveText(/开赛 09:12\s*晚 12 分钟/);
      await noOverflow(page);

      // Changed by hand.
      await first.getByRole("button", { name: "开赛时间", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "第1组开赛时间", exact: true });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByLabel("日期")).toHaveValue("2026-11-12");
      await expect(dialog.getByLabel("时间")).toHaveValue("09:12");
      await noOverflow(page);
      await dialog.getByLabel("时间").fill("08:55");
      await dialog.getByRole("button", { name: "保存", exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(first.locator("[data-started]")).toHaveText(/开赛 08:55\s*早 5 分钟/);

      // Cleared.
      await first.getByRole("button", { name: "开赛时间", exact: true }).click();
      await dialog.getByRole("button", { name: "清除", exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(first.locator("[data-started]")).toHaveCount(0);
      expect(await calls(page, "__startedCalls")).toEqual([{ heatId: "heat", startedAt: "2026-11-12T08:55" }, { heatId: "heat", startedAt: "" }]);

      // A heat without one opens on its planned date with no time.
      await page.locator(".st-card").filter({ hasText: "第3组" }).first().getByRole("button", { name: "开赛时间", exact: true }).click();
      const third = page.getByRole("dialog", { name: "第3组开赛时间", exact: true });
      await expect(third.getByLabel("日期")).toHaveValue("2026-11-12");
      await expect(third.getByLabel("时间")).toHaveValue("");
      await expect(third.getByRole("button", { name: "保存", exact: true })).toBeDisabled();
      await expect(third.getByRole("button", { name: "清除", exact: true })).toHaveCount(0);
      await third.getByRole("button", { name: "取消", exact: true }).click();

      // The schedule shows it after the planned time, whole in the tooltip.
      await page.locator(".app__side").getByRole("button", { name: "日程", exact: true }).click();
      const day = page.getByRole("region", { name: "2026-11-12", exact: true });
      const row = day.getByRole("row").filter({ hasText: "第2组" });
      const cell = row.locator("[data-started]");
      await expect(cell).toHaveAttribute("title", "开赛 09:19 晚 12 分钟");
      await expect(cell).toContainText("09:19");
      if (width === 1280) await expect(cell).toHaveText(/开赛 09:19\s*晚 12 分钟/);
      await noOverflow(page);
      expect(errors).toEqual([]);
    });

    test(`检录单与打印设置 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await openMeet(page);
      await page.locator(".app__side").getByRole("button", { name: "打印", exact: true }).click();
      await selectReport(page, "检录单");
      const frame = page.frameLocator('iframe[title="预览"]');
      await expect(frame.getByText("全部日期 · 全部轮次", { exact: true })).toBeVisible();
      const side = page.locator(".split__side");
      await expect(side.getByText("男子100m 初中组", { exact: true })).toBeVisible();
      await expect(side.getByText("男子跳远", { exact: true })).toBeVisible();
      await noOverflow(page);

      // A day lists only its rounds.
      await side.getByRole("button", { name: "日期", exact: true }).click();
      await page.getByRole("option", { name: "2026-11-13", exact: true }).click();
      await expect(side.getByText("男子100m 初中组", { exact: true })).toHaveCount(0);
      await expect(frame.getByText("2026-11-13 · 全部轮次", { exact: true })).toBeVisible();
      await side.getByRole("button", { name: "日期", exact: true }).click();
      await page.getByRole("option", { name: "全部日期", exact: true }).click();

      // Unticking a round asks for the rest.
      await side.getByText("男子跳远", { exact: true }).click();
      await expect(frame.getByText("全部日期 · round", { exact: true })).toBeVisible();
      const print = page.locator(".st-button--primary", { hasText: "打印" });
      await print.click();
      // What prints is the sheets shown.
      await expect.poll(() => calls(page, "__prints")).toHaveLength(1);
      expect((await calls(page, "__prints"))[0]).toMatch(/class="pv-page"[\s\S]*全部日期 · round/);
      // 成绩记录表 prints the same choice.
      await selectReport(page, "成绩记录表");
      await expect(frame.getByRole("heading", { name: "成绩记录表", exact: true })).toBeVisible();
      await expect(frame.getByText("全部日期 · round", { exact: true })).toBeVisible();
      await print.click();
      await expect.poll(() => calls(page, "__prints")).toHaveLength(2);
      expect((await calls(page, "__prints"))[1]).toMatch(/<h1>成绩记录表<\/h1>[\s\S]*全部日期 · round/);
      await noOverflow(page);
      await side.getByRole("button", { name: "全选", exact: true }).click();
      await expect(frame.getByText("全部日期 · 全部轮次", { exact: true })).toBeVisible();
      // The preview is on sheets, with a page count and a scale.
      await expect(page.locator(".subbar__count")).toHaveText("1 页");
      await expect(frame.locator(".pv-page")).toHaveCount(1);
      const zoom = page.getByRole("button", { name: "比例", exact: true });
      await expect(zoom).toContainText("适合宽度");
      await zoom.click();
      await page.getByRole("option", { name: "50%", exact: true }).click();
      await expect(frame.locator(".pv-pages")).toHaveAttribute("style", /scale\(0\.5\)/);
      await noOverflow(page);

      // Nothing chosen says so; choosing again brings the preview back.
      await side.getByRole("button", { name: "取消全选", exact: true }).click();
      await expect(print).toBeDisabled();
      await expect(page.getByText("未选择轮次", { exact: true })).toBeVisible();
      await noOverflow(page);
      await selectReport(page, "秩序册");
      await expect(frame.getByText("秩序册", { exact: true })).toBeVisible();
      await side.getByRole("button", { name: "取消全选", exact: true }).click();
      await expect(page.getByText("未选择项目", { exact: true })).toBeVisible();
      await side.getByRole("button", { name: "全选", exact: true }).click();
      await expect(frame.getByText("秩序册", { exact: true })).toBeVisible();
      await expect(print).toBeEnabled();

      // The settings it follows.
      await page.goto("/");
      await page.getByRole("button", { name: "设置", exact: true }).click();
      const nav = page.getByRole("navigation", { name: "设置分类" });
      await nav.getByRole("button", { name: "编排", exact: true }).click();
      const lead = page.getByRole("button", { name: "检录提前时间", exact: true });
      await expect(lead).toContainText("20 分钟");
      await lead.click();
      await page.getByRole("option", { name: "不显示", exact: true }).click();
      await expect(lead).toContainText("不显示");
      await noOverflow(page);
      if (width === 800) await page.getByRole("button", { name: "设置分类", exact: true }).click();
      await nav.getByRole("button", { name: "打印", exact: true }).click();
      const time = page.getByRole("switch", { name: /成绩时间/ });
      await expect(time).toHaveAttribute("aria-checked", "false");
      await time.click();
      await expect(time).toHaveAttribute("aria-checked", "true");
      // 分页: 检录单 first, then 成绩记录表; both by round.
      const breaks = page.getByRole("button", { name: "分页", exact: true });
      await expect(breaks).toHaveCount(2);
      for (const [i, choice] of [[0, "每组一页"], [1, "不分页"]] as const) {
        await expect(breaks.nth(i)).toContainText("每个轮次一页");
        await breaks.nth(i).click();
        await page.getByRole("option", { name: choice, exact: true }).click();
        await expect(breaks.nth(i)).toContainText(choice);
      }
      expect(await page.evaluate(async () => {
        const p = await (globalThis as any).mygo.call("Settings.Preferences");
        return [p.checkInLeadMinutes, p.printResultsTime, p.printCheckInBreak, p.printRecordBreak];
      })).toEqual([0, true, "heat", "none"]);
      await noOverflow(page);
      expect(errors).toEqual([]);
    });
  }
}
