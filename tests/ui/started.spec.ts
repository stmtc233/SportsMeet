import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

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
      await page.getByRole("radio", { name: "检录单", exact: true }).click();
      const frame = page.frameLocator("iframe.report-frame");
      await expect(frame.getByText("全部日期 · 全部轮次", { exact: true })).toBeVisible();
      const side = page.locator(".split__side");
      await expect(side.getByText("男子100m 初中组", { exact: true })).toBeVisible();
      await expect(side.getByText("男子跳远", { exact: true })).toBeVisible();
      await noOverflow(page);

      // A day lists only its rounds.
      await side.getByRole("button", { name: "日期", exact: true }).click();
      await page.getByRole("menuitemradio", { name: "2026-11-13", exact: true }).click();
      await expect(side.getByText("男子100m 初中组", { exact: true })).toHaveCount(0);
      await expect(frame.getByText("2026-11-13 · 全部轮次", { exact: true })).toBeVisible();
      await side.getByRole("button", { name: "日期", exact: true }).click();
      await page.getByRole("menuitemradio", { name: "全部日期", exact: true }).click();

      // Unticking a round asks for the rest.
      await side.getByText("男子跳远", { exact: true }).click();
      await expect(frame.getByText("全部日期 · round", { exact: true })).toBeVisible();
      const print = page.locator(".st-button--primary", { hasText: "打印" });
      await print.click();
      await expect.poll(() => calls(page, "__checkInPrints")).toEqual([{ meetId: "meet", roundIds: ["round"], day: "" }]);
      await side.getByRole("button", { name: "全选", exact: true }).click();
      await expect(frame.getByText("全部日期 · 全部轮次", { exact: true })).toBeVisible();
      await side.getByRole("button", { name: "取消全选", exact: true }).click();
      await expect(print).toBeDisabled();
      await noOverflow(page);

      // The settings it follows.
      await page.goto("/");
      await page.getByRole("button", { name: "设置", exact: true }).click();
      const nav = page.getByRole("navigation", { name: "设置分类" });
      await nav.getByRole("button", { name: "编排", exact: true }).click();
      const lead = page.getByRole("button", { name: "检录提前时间", exact: true });
      await expect(lead).toContainText("20 分钟");
      await lead.click();
      await page.getByRole("menuitemradio", { name: "不显示", exact: true }).click();
      await expect(lead).toContainText("不显示");
      await noOverflow(page);
      if (width === 800) await page.getByRole("button", { name: "设置分类", exact: true }).click();
      await nav.getByRole("button", { name: "打印", exact: true }).click();
      for (const name of ["成绩时间", "每组另起一页"]) {
        const item = page.getByRole("switch", { name: new RegExp(name) });
        await expect(item).toHaveAttribute("aria-checked", "false");
        await item.click();
        await expect(item).toHaveAttribute("aria-checked", "true");
      }
      expect(await page.evaluate(async () => {
        const p = await (globalThis as any).mygo.call("Settings.Preferences");
        return [p.checkInLeadMinutes, p.printResultsTime, p.printCheckInNewPage];
      })).toEqual([0, true, true]);
      await noOverflow(page);
      expect(errors).toEqual([]);
    });
  }
}
