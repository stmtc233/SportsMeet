import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { installRuntime, selectReport } from "./runtime";

const style = readFileSync("internal/report/style.css", "utf8");

async function installTeamScoring(page: Page, disabled = false) {
  await installRuntime(page);
  await page.addInitScript(({ disabled, style }) => {
    const bridge = (globalThis as any).mygo;
    const baseCall = bridge.call;
    const baseOn = bridge.on;
    const listeners = new Set<(c: any) => void>();
    bridge.on = (name: string, fn: any) => { listeners.add(fn); const off = baseOn(name, fn); return () => { listeners.delete(fn); off(); }; };
    const changed = () => { for (const fn of listeners) fn({ kind: "team_scoring", meetId: "meet", eventId: "" }); };
    const value = (n: number) => ({ exact: String(n), text: String(n) });
    const clone = (v: any) => structuredClone(v);
    const error = (message: string) => { const e = new Error(message); e.name = "CallError"; throw e; };
    let edited = false;
    let rules = { enabled: !disabled, places: [9, 7, 6, 5, 4, 3, 2, 1], tiePolicy: "average", relayMultiplier: 2, overrides: [] as any[] };
    const targets = [{ eventId: "event", combinedId: "", name: "男子100m 初中组", gender: "男", groupLabel: "初中组", relay: false }, { eventId: "relay", combinedId: "", name: "混合4×100m 初中组", gender: "混合", groupLabel: "初中组", relay: true }];
    const detail = (next = rules, filter: any = {}) => {
      const sources = targets.filter((t) => (!filter.gender || t.gender === filter.gender) && (filter.groupLabel === undefined || t.groupLabel === filter.groupLabel)).map((t) => ({ ...t, complete: true, multiplier: t.relay ? 2 : 1, state: next.enabled ? "已计分" : "不计分" }));
      const active = next.enabled && sources.length > 0;
      const isEdited = edited;
      const rows = ["一班", "二班"].map((unitName, i) => ({ unitId: `u${i}`, unitName, rank: active ? (isEdited ? i + 1 : 1) : null,
        male: value(active ? (i === 0 ? 9 : 7) : 0), female: value(0), mixed: value(active ? (i === 0 ? 7 : 9) : 0), total: value(active ? (isEdited ? (i === 0 ? 20 : 10) : 16) : 0),
        lines: active ? [{ eventId: "event", combinedId: "", eventName: "男子100m 初中组", competitorId: `a${i}`, athleteId: `a${i}`, teamId: "", unitId: `u${i}`, name: `运动员${i + 1}`, markText: "12.34", rank: i + 1, gender: "男", groupLabel: "初中组", base: value(9), multiplier: 1, score: value(9) }] : [] }));
      return { rules: clone(next), targets, groups: ["初中组"], rows, sources, unassigned: [], warnings: [], counted: active ? sources.length : 0, total: next.enabled ? sources.length : 0, complete: active, state: !next.enabled ? "未启用" : active ? "正式排名" : "暂列排名" };
    };
    (globalThis as any).__teamScoreCalls = [];
    bridge.call = async (method: string, ...args: any[]) => {
      if (method.startsWith("TeamScoring.") || method.startsWith("Reports.")) (globalThis as any).__teamScoreCalls.push({ method, args: clone(args) });
      switch (method) {
        case "Groups.List": return [{ id: "g1", meetId: "meet", name: "初中组", ordinal: 1, events: 2, units: 2, athletes: 2 }];
        case "TeamScoring.Boards": return [{ groupId: "g1", groupName: "初中组", detail: clone(detail(rules, args[0])) }];
        case "TeamScoring.Get": return clone(detail(rules, args[0]));
        case "TeamScoring.Save": {
          const input = args[1];
          if (!input.places.trim() || input.places.includes("x")) return error("分值和倍数须为非负整数");
          rules = { enabled: input.enabled, places: input.places.split(/[、,\s]+/).map(Number), tiePolicy: input.tiePolicy, relayMultiplier: Number(input.relayMultiplier), overrides: input.overrides.map((o: any) => ({ ...o, multiplier: Number(o.multiplier) })) };
          edited = input.places.startsWith("10"); changed();
          return clone(rules);
        }
        case "Reports.Targets": return targets.map((t) => ({ id: t.eventId, name: t.name }));
        case "Reports.Render": return `<!doctype html><html><head><style>${style}</style></head><body><header><h1>秋季运动会</h1></header><section class="event"><h2>团体总分表</h2><p>正式排名</p><table><thead><tr><th>名次</th><th>参赛单位</th><th>总分</th></tr></thead><tbody><tr><td>1</td><td>一班</td><td>16</td></tr><tr><td>1</td><td>二班</td><td>16</td></tr></tbody></table></section></body></html>`;
        case "Reports.PrintPages": return null;
        default: return baseCall(method, ...args);
      }
    };
  }, { disabled, style });
}

const calls = (page: Page) => page.evaluate(() => (globalThis as any).__teamScoreCalls);

for (const width of [1280, 800]) for (const colorScheme of ["light", "dark"] as const) {
  test(`团体总分、明细、规则与打印 ${width} ${colorScheme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ colorScheme });
    await installTeamScoring(page);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await expect(page.getByRole("heading", { name: "初中组 正式排名", exact: true })).toBeVisible();
    const table = page.getByRole("table").first();
    await expect(table.getByRole("row").nth(1).getByRole("cell").first()).toHaveText("1");
    await expect(table.getByRole("row").nth(2).getByRole("cell").first()).toHaveText("1");
    const totalCell = table.getByRole("row").nth(1).getByRole("cell").last();
    const scoreBounds = await totalCell.boundingBox();
    expect(scoreBounds!.x + scoreBounds!.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: `test-results/team-scores-${width}-${colorScheme}.png`, animations: "disabled" });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    // One gender: only the total column is left.
    await page.getByRole("radio", { name: "男子", exact: true }).click();
    await expect(page.getByRole("button", { name: "已计分 1 / 1 项", exact: true })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "女子", exact: true })).toHaveCount(0);
    await page.getByRole("radio", { name: "女子", exact: true }).click();
    await expect(page.getByText("没有符合条件的计分项目", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "运动员", exact: true }).click();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await expect(page.getByRole("radio", { name: "女子", exact: true })).toBeChecked();
    await page.getByRole("radio", { name: "全部", exact: true }).click();
    await page.getByRole("button", { name: "已计分 2 / 2 项", exact: true }).click();
    const sources = page.getByRole("dialog", { name: "初中组计分项目", exact: true });
    await expect(sources.getByText("混合4×100m 初中组", { exact: true })).toBeVisible();
    await sources.getByRole("button", { name: "关闭", exact: true }).click();
    await page.getByRole("table").first().getByText("一班", { exact: true }).click();
    const lines = page.getByRole("dialog", { name: "一班", exact: true });
    await expect(lines.getByText("运动员1", { exact: true })).toBeVisible();
    await lines.getByText("男子100m 初中组", { exact: true }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await page.getByRole("button", { name: "计分规则", exact: true }).click();
    const rules = page.getByRole("dialog", { name: "计分规则", exact: true });
    await rules.getByLabel("名次分值", { exact: true }).fill("10、8、6、5、4、3、2、1");
    await rules.getByLabel("男子100m 初中组的倍数", { exact: true }).fill("2");
    await page.screenshot({ path: `test-results/team-rules-${width}-${colorScheme}.png`, animations: "disabled" });
    await rules.getByRole("button", { name: "保存", exact: true }).click();
    await expect(rules).not.toBeVisible();
    await expect(page.getByText("20", { exact: true })).toBeVisible();
    const save = (await calls(page)).findLast((c: any) => c.method === "TeamScoring.Save").args[1];
    expect(save.overrides).toEqual([{ eventId: "event", combinedId: "", multiplier: "2" }]);
    await page.getByRole("button", { name: "打印", exact: true }).click();
    await selectReport(page, "团体总分");
    await expect(page.frameLocator('iframe[title="预览"]').getByRole("heading", { name: "团体总分表", exact: true })).toBeVisible();
    await page.getByRole("checkbox", { name: "混合4×100m 初中组", exact: true }).uncheck();
    await page.locator(".st-toolbar").getByRole("button", { name: "打印", exact: true }).click();
    expect((await calls(page)).findLast((c: any) => c.method === "Reports.Render").args).toEqual(["team_scores", "meet", ["event"]]);
    await expect.poll(async () => (await calls(page)).findLast((c: any) => c.method === "Reports.PrintPages")?.args[1] ?? "").toContain("团体总分表");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("一键启用计分，校验错误保留输入", async ({ page }) => {
  await installTeamScoring(page, true);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await selectReport(page, "团体总分");
  await expect(page.getByText("尚未启用团体计分", { exact: true })).toBeVisible();
  await expect(page.locator(".st-toolbar").getByRole("button", { name: "打印", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "团体总分", exact: true }).click();
  await expect(page.getByText("未启用团体计分", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "启用团体计分", exact: true }).click();
  await expect(page.getByRole("heading", { name: "初中组 正式排名", exact: true })).toBeVisible();
  expect((await calls(page)).findLast((c: any) => c.method === "TeamScoring.Save").args[1]).toMatchObject({ enabled: true, places: "9、7、6、5、4、3、2、1" });
  await page.getByRole("button", { name: "计分规则", exact: true }).click();
  const rules = page.getByRole("dialog", { name: "计分规则", exact: true });
  await rules.getByLabel("名次分值", { exact: true }).fill("x");
  await rules.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("分值和倍数须为非负整数", { exact: true })).toBeVisible();
  await expect(rules).toBeVisible();
  await expect(rules.getByLabel("名次分值", { exact: true })).toHaveValue("x");
});
