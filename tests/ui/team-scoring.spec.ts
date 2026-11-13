import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { installRuntime } from "./runtime";

const style = readFileSync("internal/report/style.css", "utf8");

async function installTeamScoring(page: Page, disabled = false, stale = false) {
  await installRuntime(page);
  await page.addInitScript(({ disabled, stale, style }) => {
    const bridge = (globalThis as any).mygo;
    const baseCall = bridge.call;
    const baseOn = bridge.on;
    const listeners = new Set<(c: any) => void>();
    bridge.on = (name: string, fn: any) => { listeners.add(fn); const off = baseOn(name, fn); return () => { listeners.delete(fn); off(); }; };
    const changed = () => { for (const fn of listeners) fn({ kind: "team_scoring", meetId: "meet", eventId: "" }); };
    const value = (n: number) => ({ exact: String(n), text: String(n) });
    const clone = (v: any) => structuredClone(v);
    const error = (message: string) => { const e = new Error(message); e.name = "CallError"; throw e; };
    let revision = 0;
    let edited = false;
    let rules = { enabled: !disabled, places: [9, 7, 6, 5, 4, 3, 2, 1], tiePolicy: "average", relayMultiplier: 2, overrides: [] as any[] };
    const targets = [{ eventId: "event", combinedId: "", name: "男子100m 初中组", gender: "男", groupLabel: "初中组", relay: false }, { eventId: "relay", combinedId: "", name: "混合4×100m 初中组", gender: "混合", groupLabel: "初中组", relay: true }];
    const detail = (next = rules, filter: any = {}, preview = false) => {
      const sources = targets.filter((t) => (!filter.gender || t.gender === filter.gender) && (filter.groupLabel === undefined || t.groupLabel === filter.groupLabel)).map((t) => ({ ...t, complete: true, multiplier: t.relay ? 2 : 1, state: next.enabled ? "已计分" : "不计分" }));
      const active = next.enabled && sources.length > 0;
      const isEdited = edited || preview;
      const rows = ["一班", "二班"].map((unitName, i) => ({ unitId: `u${i}`, unitName, rank: active ? (isEdited ? i + 1 : 1) : null,
        male: value(active ? (i === 0 ? 9 : 7) : 0), female: value(0), mixed: value(active ? (i === 0 ? 7 : 9) : 0), total: value(active ? (isEdited ? (i === 0 ? 20 : 10) : 16) : 0),
        lines: active ? [{ eventId: "event", combinedId: "", eventName: "男子100m 初中组", competitorId: `a${i}`, athleteId: `a${i}`, teamId: "", unitId: `u${i}`, name: `运动员${i + 1}`, markText: "12.34", rank: i + 1, gender: "男", groupLabel: "初中组", base: value(9), multiplier: 1, score: value(9) }] : [] }));
      return { rules: clone(next), targets, groups: ["初中组"], rows, sources, unassigned: [], counted: active ? sources.length : 0, total: next.enabled ? sources.length : 0, complete: active, state: !next.enabled ? "未启用" : active ? "正式排名" : "暂列排名" };
    };
    (globalThis as any).__teamScoreCalls = [];
    (globalThis as any).__teamScoreChange = () => { revision++; changed(); };
    bridge.call = async (method: string, ...args: any[]) => {
      if (method.startsWith("TeamScoring.") || method.startsWith("Reports.")) (globalThis as any).__teamScoreCalls.push({ method, args: clone(args) });
      switch (method) {
        case "TeamScoring.Get": return clone(detail(rules, args[0]));
        case "TeamScoring.Preview": {
          const input = args[1];
          if (!input.places.trim() || input.places.includes("x")) return error("分值和倍数须为非负整数");
          const next = { enabled: input.enabled, places: input.places.split(/[、,\s]+/).map(Number), tiePolicy: input.tiePolicy, relayMultiplier: Number(input.relayMultiplier), overrides: input.overrides.map((o: any) => ({ ...o, multiplier: Number(o.multiplier) })) };
          return { previewHash: String(revision), changed: true, before: detail(), after: detail(next, {}, true) };
        }
        case "TeamScoring.Apply":
          if (stale || args[1] !== String(revision)) { stale = false; return error("E_STALE"); }
          rules = args[0].rules; edited = true; revision++; changed();
          return { changed: true };
        case "Reports.Targets": return targets.map((t) => ({ id: t.eventId, name: t.name }));
        case "Reports.Render": return `<!doctype html><html><head><style>${style}</style></head><body><header><h1>秋季运动会</h1></header><section class="event"><h2>团体总分表</h2><p>正式排名</p><table><thead><tr><th>名次</th><th>参赛单位</th><th>总分</th></tr></thead><tbody><tr><td>1</td><td>一班</td><td>16</td></tr><tr><td>1</td><td>二班</td><td>16</td></tr></tbody></table></section></body></html>`;
        case "Reports.Print": return null;
        default: return baseCall(method, ...args);
      }
    };
  }, { disabled, stale, style });
}

for (const width of [1280, 800]) for (const colorScheme of ["light", "dark"] as const) {
  test(`团体总分、明细、规则预览与打印 ${width} ${colorScheme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ colorScheme });
    await installTeamScoring(page);
    await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await expect(page.getByRole("heading", { name: "正式排名", exact: true })).toBeVisible();
    const table = page.getByRole("table").first();
    await expect(table.getByRole("row").nth(1).getByRole("cell").first()).toHaveText("1");
    await expect(table.getByRole("row").nth(2).getByRole("cell").first()).toHaveText("1");
    const totalCell = table.getByRole("row").nth(1).getByRole("cell").last();
    const scoreBounds = await totalCell.boundingBox();
    expect(scoreBounds!.x + scoreBounds!.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: `test-results/team-scores-${width}-${colorScheme}.png`, animations: "disabled" });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByLabel("性别", { exact: true }).click();
    await page.getByRole("menuitemradio", { name: "女子", exact: true }).click();
    await expect(page.getByRole("heading", { name: "暂列排名", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "运动员", exact: true }).click();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await expect(page.getByLabel("性别", { exact: true })).toHaveText(/女子/);
    await page.getByLabel("性别", { exact: true }).click();
    await page.getByRole("menuitemradio", { name: "全部性别", exact: true }).click();
    await page.getByLabel("组别", { exact: true }).click();
    await page.getByRole("menuitemradio", { name: "初中组", exact: true }).click();
    await expect(page.getByRole("heading", { name: "正式排名", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "一班", exact: true }).press("Enter");
    const lines = page.getByRole("dialog", { name: "一班", exact: true });
    await expect(lines.getByText("运动员1", { exact: true })).toBeVisible();
    await lines.getByRole("button", { name: "男子100m 初中组", exact: true }).click();
    await expect(page.getByRole("radio", { name: "成绩", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await page.getByRole("button", { name: "计分规则", exact: true }).click();
    const rules = page.getByRole("dialog", { name: "团体计分规则", exact: true });
    await expect(rules.getByRole("button", { name: "保存规则", exact: true })).toBeDisabled();
    await expect(rules.getByText("比较名次数量")).toHaveCount(0);
    await rules.getByLabel("名次分值", { exact: true }).fill("10、8、6、5、4、3、2、1");
    await rules.getByLabel("男子100m 初中组计分倍数", { exact: true }).fill("2");
    await rules.getByRole("button", { name: "预览重算", exact: true }).click();
    await expect(rules.getByRole("heading", { name: "重算预览", exact: true })).toBeVisible();
    await expect(rules.getByText("20", { exact: true })).toBeVisible();
    await page.screenshot({ path: `test-results/team-rules-${width}-${colorScheme}.png`, animations: "disabled" });
    await page.evaluate(() => (globalThis as any).__teamScoreChange());
    await expect(rules.getByRole("button", { name: "保存规则", exact: true })).toBeDisabled();
    await expect(rules.getByLabel("名次分值", { exact: true })).toHaveValue("10、8、6、5、4、3、2、1");
    await rules.getByRole("button", { name: "预览重算", exact: true }).click();
    await rules.getByRole("button", { name: "保存规则", exact: true }).click();
    await expect(rules).not.toBeVisible();
    await expect(page.getByText("20", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "打印", exact: true }).click();
    await page.getByRole("radio", { name: "团体总分", exact: true }).click();
    await expect(page.frameLocator('iframe[title="预览"]').getByRole("heading", { name: "团体总分表", exact: true })).toBeVisible();
    await page.getByRole("checkbox", { name: "混合4×100m 初中组", exact: true }).uncheck();
    await page.locator(".st-toolbar").getByRole("button", { name: "打印", exact: true }).click();
    const calls = await page.evaluate(() => (globalThis as any).__teamScoreCalls);
    expect(calls.findLast((c: any) => c.method === "Reports.Print").args).toEqual(["team_scores", "meet", ["event"]]);
    expect(calls.findLast((c: any) => c.method === "TeamScoring.Apply").args[2]).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("启用计分、校验错误和过期保存保留输入", async ({ page }) => {
  await installTeamScoring(page, true, true);
  await page.goto("/");
  await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("button", { name: "团体总分", exact: true }).click();
  await expect(page.getByText("尚未启用团体计分", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "打印", exact: true }).click();
  await page.getByRole("radio", { name: "团体总分", exact: true }).click();
  await expect(page.getByText("尚未启用团体计分", { exact: true })).toBeVisible();
  await expect(page.locator(".st-toolbar").getByRole("button", { name: "打印", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "团体总分", exact: true }).click();
  await page.getByRole("button", { name: "设置计分规则", exact: true }).click();
  const rules = page.getByRole("dialog", { name: "团体计分规则", exact: true });
  await rules.getByRole("switch", { name: "启用团体计分", exact: true }).click();
  await rules.getByLabel("名次分值", { exact: true }).fill("x");
  await rules.getByRole("button", { name: "预览重算", exact: true }).click();
  await expect(page.getByText("分值和倍数须为非负整数", { exact: true })).toBeVisible();
  await expect(rules.getByLabel("名次分值", { exact: true })).toHaveValue("x");
  await rules.getByLabel("名次分值", { exact: true }).fill("9、7、6、5、4、3、2、1");
  await rules.getByRole("button", { name: "预览重算", exact: true }).click();
  await rules.getByRole("button", { name: "保存规则", exact: true }).click();
  await expect(rules).toBeVisible();
  await expect(rules.getByRole("button", { name: "保存规则", exact: true })).toBeDisabled();
  await expect(rules.getByLabel("名次分值", { exact: true })).toHaveValue("9、7、6、5、4、3、2、1");
  await rules.getByRole("button", { name: "预览重算", exact: true }).click();
  await rules.getByRole("button", { name: "保存规则", exact: true }).click();
  await expect(rules).not.toBeVisible();
});
