import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function noOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

async function openTimetable(page: Page) {
  await installRuntime(page, { schedule: true, conflicts: true });
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.locator(".app__side").getByRole("button", { name: "日程", exact: true }).click();
  await expect(page.getByRole("heading", { name: "日程", exact: true })).toBeVisible();
}

for (const width of [1280, 800]) {
  for (const colorScheme of ["light", "dark"] as const) {
    test(`日程页按天列出与冲突 ${width} ${colorScheme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.emulateMedia({ colorScheme });
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await openTimetable(page);
      const day = page.getByRole("region", { name: "2026-11-12", exact: true });
      await expect(day.getByText("男子100m 初中组 决赛", { exact: true })).toBeVisible();
      await expect(day.getByText("男子跳远 决赛", { exact: true })).toBeVisible();
      await expect(page.getByRole("region", { name: "未安排时间", exact: true }).getByText("男子跳高 决赛", { exact: true })).toBeVisible();
      const conflicts = page.locator("[data-conflicts]");
      await expect(conflicts.getByText("运动员1", { exact: true })).toBeVisible();
      await expect(conflicts.getByText("时间重叠", { exact: true })).toBeVisible();
      await expect(day.locator("[data-conflict]")).toHaveCount(2);
      await noOverflow(page);
      await page.locator(".app__side").getByRole("button", { name: "日程", exact: true }).click();
      await page.getByRole("button", { name: "连排…", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "连排轮次", exact: true })).toBeVisible();
      await noOverflow(page);
      expect(errors).toEqual([]);
    });
  }
}

test("日程页修改组时间", async ({ page }) => {
  await openTimetable(page);
  const sprint = page.locator(".st-card").filter({ hasText: "男子100m 初中组 决赛" });
  await sprint.getByRole("row").filter({ hasText: "第3组" }).getByRole("button", { name: "编辑组", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "编辑组", exact: true });
  await dialog.getByLabel("时间", { exact: true }).fill("10:00");
  await dialog.getByLabel("预计用时").fill("5");
  await dialog.getByRole("button", { name: "确定", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(sprint.getByText("10:00", { exact: true })).toBeVisible();
  const edits = await page.evaluate(() => (globalThis as any).__heatEdits);
  expect(edits).toEqual([{ eventId: "event", heatId: "later", base: "later-base", name: "", scheduledAt: "2026-11-12T10:00", durationMinutes: 5 }]);
});

test("日程页修改轮次开始时间", async ({ page }) => {
  await openTimetable(page);
  const sprint = page.locator(".st-card").filter({ hasText: "男子100m 初中组 决赛" });
  await sprint.getByRole("button", { name: "开始时间", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "男子100m 初中组 决赛开始时间", exact: true });
  await dialog.getByLabel("时间", { exact: true }).fill("08:30");
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => (globalThis as any).__roundStarts)).toEqual(["2026-11-12T08:30"]);
  await expect(sprint.getByRole("button", { name: "08:30", exact: true })).toBeVisible();
});

test("多轮次连排预览冲突后保存", async ({ page }) => {
  await openTimetable(page);
  await page.getByRole("button", { name: "连排…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "连排轮次", exact: true });
  await expect(dialog.getByRole("button", { name: "预览", exact: true })).toBeDisabled();
  await dialog.getByText("男子100m 初中组 决赛", { exact: true }).click();
  await dialog.getByText("男子跳远 决赛", { exact: true }).click();
  await dialog.getByLabel("轮次间隔").fill("10");
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await expect(dialog.getByText("排完后有 1 处时间冲突", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "冲突 1 处", exact: true })).toBeVisible();
  const request = await page.evaluate(() => (globalThis as any).__scheduleCalls.request);
  expect(request.rounds).toEqual([{ eventId: "event", roundId: "round" }, { eventId: "jump", roundId: "jump-round" }]);
  expect(request.roundGapMinutes).toBe(10);
  // A changed field drops the confirmation.
  await dialog.getByLabel("轮次间隔").fill("5");
  await expect(dialog.getByRole("button", { name: "保存", exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "预览", exact: true }).click();
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => (globalThis as any).__scheduleCalls.save)).toBe(1);
});

test("从冲突跳到编排页并在组上标出", async ({ page }) => {
  await openTimetable(page);
  await page.locator("[data-conflicts]").getByRole("button", { name: /男子100m 初中组 决赛 第1组/ }).click();
  await expect(page.getByRole("radio", { name: "编排", exact: true })).toBeChecked();
  const tag = page.locator(".heats [data-conflict]");
  await expect(tag).toHaveCount(1);
  await expect(tag).toHaveAttribute("title", /运动员1：时间重叠，2026-11-12 09:00 男子跳远 决赛 第1组/);
});

test("设置里的最短休息时间与竞赛日程表", async ({ page }) => {
  await installRuntime(page);
  await page.goto("/");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  const nav = page.getByRole("navigation", { name: "设置分类" });
  await nav.getByRole("button", { name: "编排", exact: true }).click();
  await expect(page.getByText("最短休息时间", { exact: true })).toBeVisible();
  await expect(page.getByText("只查重叠", { exact: true })).toBeVisible();
  await nav.getByRole("button", { name: "打印", exact: true }).click();
  await expect(page.getByText("竞赛日程表", { exact: true })).toBeVisible();
});
