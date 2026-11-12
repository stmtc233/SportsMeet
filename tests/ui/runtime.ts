import type { Page } from "@playwright/test";

// Page tests exercise the UI with a deterministic mygo bridge. Business writes
// and actual HTTP MCP calls are verified against real SQLite by the Go tests.
export async function installRuntime(page: Page, options: { schedule?: boolean; staleSchedule?: boolean; scheduleErrors?: boolean } = {}) {
  await page.addInitScript((options) => {
    const listeners = new Set<(change: unknown) => void>();
    let prefs = { theme: "system", separateUnits: true, balance: true, printNewPage: false };
    let mcp = { enabled: false, running: false, port: 18765, address: "http://127.0.0.1:18765/mcp", error: "" };
    const meet = { id: "meet", name: "秋季运动会", startDate: "2026-11-12", endDate: "", location: "体育场", note: "", createdAt: 1 };
    const event = { id: "event", meetId: "meet", name: "男子100m 初中组", gender: "男", discipline: "100m", groupLabel: "初中组", kind: "track", date: "", laneCount: 8, attemptCount: 3, note: "" };
    const athletes = Array.from({ length: 24 }, (_, i) => ({ id: `a${i}`, meetId: "meet", name: `运动员${i + 1}`, gender: "男", bib: String(100 + i), studentNumber: "", unitId: "unit", className: "一班", grade: "", birthDate: "", note: "", createdAt: 1 }));
    const entries = athletes.map((a) => ({ athleteId: a.id, name: a.name, gender: "男", bib: a.bib, unitId: "unit", unitName: "一班", seedMark: null, seedText: "", placed: false, warning: "" }));
    const detail = { event, entries, rounds: [{ id: "round", stage: "final", ordinal: 1, name: "", label: "决赛", advance: null, complete: false, standings: [], heats: [{ id: "heat", ordinal: 1, name: "", label: "第1组", scheduledAt: "2026-11-12T09:00", base: "base", keepReason: "", slots: entries.slice(0, 8).map((a, i) => ({ position: i + 1, athleteId: a.athleteId, result: null })) }] }] };
    if (options.schedule) {
      detail.rounds[0]!.heats.push(
        { id: "scored", ordinal: 2, name: "", label: "第2组", scheduledAt: "2026-11-12T09:07", base: "scored-base", keepReason: "已有成绩", slots: [] },
        { id: "later", ordinal: 3, name: "", label: "第3组", scheduledAt: "", base: "later-base", keepReason: "", slots: [] },
      );
    }
    const changed = (kind = "settings", meetId = "", eventId = "") => { for (const fn of listeners) fn({ kind, meetId, eventId }); };
    let staleSchedule = options.staleSchedule;
    (globalThis as any).__scheduleCalls = { preview: 0, save: 0 };
    (globalThis as any).__scheduleExternalChange = () => changed("heats", "meet", "event");
    const schedulePreview = {
      previewHash: "schedule-hash", errors: [] as string[],
      changes: [
        { heatId: "heat", label: "第1组", before: "2026-11-12T09:00", after: "2026-11-12T23:58", beforeText: "2026-11-12 09:00", afterText: "2026-11-12 23:58" },
        { heatId: "later", label: "第3组", before: "", after: "2026-11-13T00:01", beforeText: "未安排", afterText: "2026-11-13 00:01" },
      ],
      skipped: [{ heatId: "scored", label: "第2组", reason: "已有成绩" }],
    };
    const error = (message: string) => { const e = new Error(message); e.name = "CallError"; throw e; };
    (globalThis as any).mygo = {
      on: (_: string, fn: (change: unknown) => void) => { listeners.add(fn); return () => listeners.delete(fn); },
      once: () => () => {},
      call: async (method: string, ...args: any[]) => {
        switch (method) {
          case "Settings.Preferences": return { ...prefs };
          case "Settings.SavePreferences": prefs = { ...args[0] }; changed(); return null;
          case "Settings.MCPStatus": return { ...mcp };
          case "Settings.ConfigureMCP":
            if (args[1] === 9999) return error("端口无法监听，请更换端口");
            mcp = { enabled: args[0], running: args[0], port: args[1], address: `http://127.0.0.1:${args[1]}/mcp`, error: "" }; changed(); return null;
          case "Settings.RevealToken": return "test-token-only";
          case "Settings.CopyToken": case "Settings.CopyConnection": return null;
          case "App.Info": return { version: "0.1.0" };
          case "Meets.List": return [meet];
          case "Meets.Get": return meet;
          case "Units.List": return [{ id: "unit", meetId: "meet", name: "一班", shortName: "" }];
          case "Athletes.List": return athletes.map((athlete) => ({ athlete, unitName: "一班", entries: 1, placed: true }));
          case "Events.List": return [{ event, autoName: true, prelim: false, advance: null, entries: 24, heats: 1, placed: 8, results: 0 }];
          case "Events.Get": return structuredClone(detail);
          case "Arrange.PreviewSchedule":
            (globalThis as any).__scheduleCalls.preview++;
            return options.scheduleErrors ? { previewHash: "", changes: [], skipped: [], errors: ["间隔须为 1–1440 分钟"] } : structuredClone(schedulePreview);
          case "Arrange.Schedule":
            (globalThis as any).__scheduleCalls.save++;
            if (staleSchedule) { staleSchedule = false; return error("E_STALE"); }
            for (const change of schedulePreview.changes) {
              const heat = detail.rounds[0]!.heats.find((h) => h.id === change.heatId)!;
              heat.scheduledAt = change.after;
              heat.base += "-scheduled";
            }
            changed("heats", "meet", "event");
            return structuredClone(schedulePreview);
          case "Reports.Render": return "<!DOCTYPE html><html><body><h1>秋季运动会</h1><p>秩序册</p></body></html>";
          case "Marks.EventName": return "男子100m";
          default: return error(`未模拟的方法：${method}`);
        }
      },
    };
  }, options);
}
