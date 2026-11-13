import { expect, test, type Page } from "@playwright/test";
import { installRuntime } from "./runtime";

async function installGroups(page: Page) {
  await installRuntime(page);
  await page.addInitScript(() => {
    const bridge = (globalThis as any).mygo;
    const base = bridge.call, baseOn = bridge.on;
    const listeners = new Set<(c: any) => void>();
    bridge.on = (name: string, fn: any) => { listeners.add(fn); const off = baseOn(name, fn); return () => { listeners.delete(fn); off(); }; };
    const changed = () => { for (const fn of listeners) fn({ kind: "groups", meetId: "meet", eventId: "" }); };
    let groups = [{ id: "g1", meetId: "meet", name: "初一", ordinal: 1, events: 1, units: 1, athletes: 1 }, { id: "g2", meetId: "meet", name: "初二", ordinal: 2, events: 1, units: 1, athletes: 1 }];
    const clone = (v: any) => structuredClone(v);
    const val = (n: number) => ({ exact: String(n), text: String(n) });
    const rules = { enabled: true, places: [9, 7], tiePolicy: "average", relayMultiplier: 2, overrides: [] };
    const board = (g: any, i: number) => ({ groupId: g.id, groupName: g.name, detail: { rules, targets: [], groups: [], rows: [{ unitId: `u${i}`, unitName: `${g.name}1班`, rank: 1, male: val(i ? 9 : 16), female: val(0), mixed: val(0), total: val(i ? 9 : 16), lines: [] }], sources: [], warnings: [], unassigned: [], state: "正式排名", counted: 1, total: 1, complete: true } });
    (globalThis as any).__groupCalls = [];
    bridge.call = async (method: string, ...args: any[]) => {
      (globalThis as any).__groupCalls.push({ method, args: clone(args) });
      switch (method) {
        case "Groups.List": return clone(groups);
        case "Groups.Save": {
          const [, id, input] = args;
          if (groups.some((g) => g.id !== id && g.name === input.name.trim())) throw new Error("已有同名组别");
          if (id) groups = groups.map((g) => g.id === id ? { ...g, name: input.name.trim() } : g);
          else groups.push({ id: "g3", meetId: "meet", name: input.name.trim(), ordinal: groups.length + 1, events: 0, units: 0, athletes: 0 });
          changed(); return clone(groups.find((g) => g.id === (id || "g3")));
        }
        case "Groups.Reorder": groups = args[1].map((id: string, i: number) => ({ ...groups.find((g) => g.id === id), ordinal: i + 1 })); changed(); return null;
        case "Groups.Delete": groups = groups.filter((g) => g.id !== args[1]); changed(); return null;
        case "Groups.Suggestions": return [{ kind: "athlete", id: "a0", name: "运动员1", candidates: ["g1"], reason: "年级" }];
        case "Groups.Assign": case "Groups.Change": changed(); return null;
        case "TeamScoring.Boards": return groups.slice(0, 2).map(board);
        case "TeamScoring.Get": return board(groups[0], 0).detail;
        case "Events.List": {
          const rows = await base(method, ...args);
          return ["g1", "g2"].map((groupId, i) => ({ ...rows[0], entries: 0, heats: 0, placed: 0, event: { ...rows[0].event, id: i ? "event2" : "event", groupId, groupLabel: groups.find((g) => g.id === groupId)?.name, name: `男子100m ${groups.find((g) => g.id === groupId)?.name}` } }));
        }
        case "Reports.Targets": return groups.slice(0, 2).map((g, i) => ({ id: i ? "event2" : "event", groupId: g.id, name: `男子100m ${g.name}` }));
        case "Reports.RenderGrouped": return `<!doctype html><html><body><h1>分组成绩册</h1><p>${args[0].groupIds.join(",")}</p></body></html>`;
        case "Reports.PrintGrouped": return null;
        default: return base(method, ...args);
      }
    };
  });
}

const lastCall = (page: Page, method: string) => page.evaluate((m) => (globalThis as any).__groupCalls.findLast((c: any) => c.method === m)?.args, method);

for (const width of [1280, 800]) for (const colorScheme of ["light", "dark"] as const) {
  test(`赛事统一组别、整理归属及独立榜单 ${width} ${colorScheme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 }); await page.emulateMedia({ colorScheme });
    await installGroups(page); await page.goto("/");
    await page.getByText("秋季运动会", { exact: true }).click();
    await page.getByRole("navigation").getByRole("button", { name: "组别", exact: true }).click();
    await page.getByRole("button", { name: "添加组别", exact: true }).click();
    const add = page.getByRole("dialog", { name: "添加组别", exact: true });
    await add.getByLabel("名称", { exact: true }).fill("初三");
    await add.getByRole("button", { name: "保存", exact: true }).click();
    const row = (name: string) => page.locator(".st-list-item").filter({ hasText: name });
    await expect(row("初三")).toBeVisible();
    await row("初三").getByRole("button", { name: "上移", exact: true }).click();
    await expect.poll(() => lastCall(page, "Groups.Reorder").then((a) => a?.[1])).toEqual(["g1", "g3", "g2"]);
    await row("初三").getByRole("button", { name: "编辑", exact: true }).click();
    const edit = page.getByRole("dialog", { name: "编辑组别", exact: true });
    await edit.getByLabel("名称", { exact: true }).fill("九年级"); await edit.getByRole("button", { name: "保存", exact: true }).click();
    // A group in use can be deleted too; the dialog says what becomes ungrouped.
    await row("初一").getByRole("button", { name: "删除", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "删除「初一」？", exact: true })).toContainText("1 个项目、1 个单位、1 名运动员将改为未分组");
    await page.getByRole("dialog", { name: "删除「初一」？", exact: true }).getByRole("button", { name: "取消", exact: true }).click();
    await row("九年级").getByRole("button", { name: "删除", exact: true }).click();
    await page.getByRole("dialog", { name: "删除「九年级」？", exact: true }).getByRole("button", { name: "删除", exact: true }).click();
    await expect(row("九年级")).toHaveCount(0);
    await page.getByRole("button", { name: "整理归属", exact: true }).click();
    const organize = page.getByRole("dialog", { name: "整理归属", exact: true });
    await expect(organize.getByLabel("运动员1的组别", { exact: true })).toHaveText("初一");
    await page.screenshot({ path: `test-results/groups-organize-${width}-${colorScheme}.png`, animations: "disabled" });
    await organize.getByRole("button", { name: "保存 1 项", exact: true }).click();
    await expect(organize).not.toBeVisible();
    expect(await lastCall(page, "Groups.Assign")).toEqual([{ meetId: "meet", assignments: [{ kind: "athlete", id: "a0", groupIds: ["g1"] }] }]);
    await page.getByRole("button", { name: "团体总分", exact: true }).click();
    await expect(page.getByRole("heading", { name: "初一 正式排名", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "初二 正式排名", exact: true })).toBeVisible();
    await page.getByLabel("组别", { exact: true }).click(); await page.getByRole("menuitemradio", { name: "未分组", exact: true }).click();
    await expect(page.getByText("没有符合条件的计分项目", { exact: true })).toBeVisible();
    await page.getByLabel("组别", { exact: true }).click(); await page.getByRole("menuitemradio", { name: "全部组别", exact: true }).click();
    await expect(page.getByText("16", { exact: true }).last()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: `test-results/groups-boards-${width}-${colorScheme}.png`, animations: "disabled" });
    await page.getByRole("button", { name: "打印", exact: true }).click(); await page.getByRole("radio", { name: "成绩册", exact: true }).click();
    const printGroups = page.getByLabel("组别", { exact: true });
    // Ticking keeps the menu open; ticking every group means all again.
    await printGroups.click(); await page.getByRole("menuitemcheckbox", { name: "初一", exact: true }).click();
    await page.getByRole("menuitemcheckbox", { name: "初二", exact: true }).click();
    await expect(printGroups).toHaveText("全部组别");
    await page.getByRole("menuitemcheckbox", { name: "初一", exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(printGroups).toHaveText("初一");
    await expect(page.getByRole("checkbox", { name: "男子100m 初二", exact: true })).toHaveCount(0);
    await expect(page.frameLocator('iframe[title="预览"]').getByRole("heading", { name: "分组成绩册" })).toBeVisible();
    await page.locator(".st-toolbar").getByRole("button", { name: "打印", exact: true }).click();
    expect((await lastCall(page, "Reports.PrintGrouped"))[0]).toMatchObject({ kind: "results", meetId: "meet", groupIds: ["g1"] });
  });
}

test("项目按组别筛选、添加并直接调整组别", async ({ page }) => {
  await installGroups(page); await page.goto("/"); await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByLabel("组别", { exact: true }).click(); await page.getByRole("menuitemradio", { name: "初二", exact: true }).click();
  await expect(page.getByText("男子100m 初一", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "添加项目", exact: true }).click();
  const add = page.getByRole("dialog", { name: "添加项目", exact: true });
  await expect(add.getByLabel("组别", { exact: true })).toHaveText("初二");
  await add.getByLabel("项目名", { exact: true }).fill("200m"); await add.getByRole("button", { name: "保存", exact: true }).click();
  expect((await lastCall(page, "Events.Save"))[2].groupId).toBe("g2");
  await page.getByRole("table").getByRole("checkbox").first().check();
  await page.getByRole("button", { name: "调整组别", exact: true }).click();
  await page.getByRole("menuitem", { name: "初一", exact: true }).click();
  expect(await lastCall(page, "Groups.Change")).toEqual([{ meetId: "meet", groupId: "g1", eventIds: ["event2"], combinedIds: [] }]);
});

test("参赛单位表格多选后调整组别", async ({ page }) => {
  await installGroups(page); await page.goto("/"); await page.getByText("秋季运动会", { exact: true }).click();
  await page.getByRole("navigation").getByRole("button", { name: "参赛单位", exact: true }).click();
  await expect(page.getByText(/^选择/)).toHaveCount(0);
  await page.getByRole("table").getByRole("row").nth(1).getByRole("checkbox").check();
  await page.getByRole("button", { name: "调整组别", exact: true }).click();
  await page.screenshot({ path: "test-results/units-group-menu.png", animations: "disabled" });
  await page.getByRole("menuitem", { name: "初二", exact: true }).click();
  const [request] = await lastCall(page, "Groups.Assign");
  expect(request.assignments).toEqual([expect.objectContaining({ kind: "unit", groupIds: ["g2"] })]);
});
