import type { Page } from "@playwright/test";

// Page tests exercise the UI with a deterministic mygo bridge. Business writes
// and actual HTTP MCP calls are verified against real SQLite by the Go tests.
export async function installRuntime(page: Page, options: { schedule?: boolean; staleSchedule?: boolean; scheduleErrors?: boolean; relay?: boolean; rejectTeamSave?: boolean; eventConfig?: "group" | "throw"; emptyBoard?: boolean } = {}) {
  await page.addInitScript((options) => {
    const listeners = new Set<(change: unknown) => void>();
    let prefs = { theme: "system", separateUnits: true, balance: true, printNewPage: false };
    let mcp = { enabled: false, running: false, port: 18765, address: "http://127.0.0.1:18765/mcp", error: "" };
    const meet = { id: "meet", name: "秋季运动会", startDate: "2026-11-12", endDate: "", location: "体育场", note: "", createdAt: 1 };
    const event = { id: "event", meetId: "meet", name: "男子100m 初中组", gender: "男", discipline: "100m", groupLabel: "初中组", kind: "track", date: "", laneCount: 8, attemptCount: 3, note: "", startMode: "lanes", distanceMode: "", relayLegCount: 0 };
    const athletes = Array.from({ length: 24 }, (_, i) => ({ id: `a${i}`, meetId: "meet", name: `运动员${i + 1}`, gender: "男", bib: String(100 + i), studentNumber: "", unitId: "unit", className: "一班", grade: "", birthDate: "", note: "", createdAt: 1 }));
    const entries = athletes.map((a) => ({ athleteId: a.id, competitorId: a.id, name: a.name, gender: "男", bib: a.bib, unitId: "unit", unitName: "一班", seedMark: null, seedText: "", placed: false, warning: "" }));
    const rules = { timed: true, usesLanes: true, positionLabel: "道次", attemptLabel: "", startLabel: "分道", defaultPerHeat: 8 };
    const detail = { rules, event, teams: [] as any[], entries, rounds: [{ id: "round", stage: "final", ordinal: 1, name: "", label: "决赛", advance: null, complete: false, standings: [], heats: [{ id: "heat", ordinal: 1, name: "", label: "第1组", scheduledAt: "2026-11-12T09:00", base: "base", keepReason: "", slots: entries.slice(0, 8).map((a, i) => ({ position: i + 1, athleteId: a.athleteId, competitorId: a.competitorId, result: null })) }] }] };
    if (options.relay) {
      Object.assign(event, { relayLegCount: 4, kind: "relay", name: "男子4×100m", discipline: "4×100m", groupLabel: "" });
      detail.entries.length = 0;
      detail.rounds[0]!.heats.length = 0;
    }
    if (options.eventConfig === "group") {
      Object.assign(event, { discipline: "1500m", name: "男子1500m", startMode: "group" });
      Object.assign(rules, { usesLanes: false, positionLabel: "顺序", startLabel: "不分道", defaultPerHeat: 12 });
      detail.rounds[0]!.heats[0]!.slots = entries.slice(0, 16).map((a, i) => ({ position: i + 1, athleteId: a.athleteId, competitorId: a.competitorId, result: null }));
    }
    if (options.eventConfig === "throw") {
      Object.assign(event, { discipline: "铅球", name: "男子铅球", kind: "distance", startMode: "", distanceMode: "throw" });
      Object.assign(rules, { timed: false, usesLanes: false, positionLabel: "顺序", attemptLabel: "试投", startLabel: "", defaultPerHeat: 12 });
    }
    if (options.emptyBoard) detail.rounds[0]!.heats[0]!.slots = [];
    const presets = [
      { id: "1500m", category: "中长跑", label: "1500m", input: { discipline: "1500m", gender: "男", kind: "track", startMode: "group", laneCount: 8, attemptCount: 3 } },
      { id: "铅球", category: "投掷", label: "铅球", input: { discipline: "铅球", gender: "男", kind: "distance", distanceMode: "throw", laneCount: 8, attemptCount: 3 } },
      { id: "混合4×400m", category: "接力", label: "混合4×400m", input: { discipline: "4×400m", gender: "混合", kind: "relay", startMode: "break", relayLegCount: 4, laneCount: 8, attemptCount: 3 } },
    ];
    (globalThis as any).__eventCalls = [];
    (globalThis as any).__relayCalls = [];
    if (options.schedule) {
      detail.rounds[0]!.heats.push(
        { id: "scored", ordinal: 2, name: "", label: "第2组", scheduledAt: "2026-11-12T09:07", base: "scored-base", keepReason: "已有成绩", slots: [] },
        { id: "later", ordinal: 3, name: "", label: "第3组", scheduledAt: "", base: "later-base", keepReason: "", slots: [] },
      );
    }
    const changed = (kind = "settings", meetId = "", eventId = "") => { for (const fn of listeners) fn({ kind, meetId, eventId }); };
    (globalThis as any).__changeToGroupStart = () => {
      Object.assign(event, { startMode: "group" });
      Object.assign(rules, { usesLanes: false, positionLabel: "顺序", startLabel: "不分道", defaultPerHeat: 12 });
      changed("events", "meet", "event");
    };
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
          case "Events.Presets": return presets;
          case "Events.Rules": {
            const e = args[0];
            const timed = e.kind === "track" || e.kind === "relay";
            const usesLanes = timed && e.startMode !== "group";
            return { timed, usesLanes, positionLabel: usesLanes ? "道次" : "顺序", attemptLabel: e.kind === "distance" ? (e.distanceMode === "throw" ? "试投" : "试跳") : "", defaultPerHeat: usesLanes ? e.laneCount : 12 };
          }
          case "Events.TeamSizeWarning": return args[0].relayLegCount > 0 && args[0].relayLegCount !== args[1] ? "棒数与项目建议不符" : "";
          case "Events.Save": {
            (globalThis as any).__eventCalls.push(structuredClone(args));
            Object.assign(event, args[2], { name: args[2].name || `${args[2].gender === "混合" ? "混合" : args[2].gender === "女" ? "女子" : "男子"}${args[2].discipline}` });
            changed("events", "meet", "event"); return structuredClone(event);
          }
          case "Events.Get": return structuredClone(detail);
          case "Events.SaveTeam": {
            (globalThis as any).__relayCalls.push({ method, args });
            if (options.rejectTeamSave) return error("队伍已有比赛记录，不能改棒次");
            const input = args[2];
            const team = { id: args[1] || `team${detail.teams.length}`, eventId: event.id, ...input,
              sizeWarning: event.relayLegCount > 0 && input.memberIds.length !== event.relayLegCount ? "棒数与项目建议不符" : "", memberNames: input.memberIds.map((id: string) => athletes.find((a) => a.id === id)?.name), entered: false, placed: false, keepReason: "", genderMismatch: false };
            const old = detail.teams.findIndex((t) => t.id === team.id);
            if (old >= 0) detail.teams[old] = { ...detail.teams[old], ...team };
            else detail.teams.push(team);
            changed("teams", "meet", "event"); return structuredClone(team);
          }
          case "Entries.Add": {
            (globalThis as any).__relayCalls.push({ method, args });
            for (const en of args[1]) {
              const team = detail.teams.find((t) => t.id === en.teamId)!;
              team.entered = true;
              detail.entries.push({ athleteId: "", competitorId: team.id, teamId: team.id, name: team.name, gender: "", bib: "", unitId: team.unitId, unitName: "一班", seedMark: null, seedText: "", placed: false, warning: "" } as any);
            }
            changed("entries", "meet", "event"); return null;
          }
          case "Arrange.Plan":
            (globalThis as any).__relayCalls.push({ method, args });
            return [{ slots: detail.entries.map((e, i) => ({ position: 4 + i, competitorId: e.competitorId })) }];
          case "Arrange.Save": {
            (globalThis as any).__relayCalls.push({ method, args });
            detail.rounds[0]!.heats = args[0].rounds[0].added.map((h: any, i: number) => ({ id: `heat${i}`, ordinal: i + 1, name: "", label: `第${i + 1}组`, base: `saved${i}`, keepReason: "", scheduledAt: "", slots: h.slots.map((sl: any) => ({ ...sl, athleteId: options.relay ? "" : sl.competitorId, teamId: options.relay ? sl.competitorId : "", result: null })) }));
            for (const t of detail.teams) t.placed = true;
            changed("heats", "meet", "event"); return structuredClone(detail);
          }
          case "Marks.Normalize": return args[1];
          case "Results.Save": {
            (globalThis as any).__relayCalls.push({ method, args });
            const h = detail.rounds[0]!.heats.find((h) => h.id === args[0])!;
            for (const input of args[1]) {
              const sl = h.slots.find((sl) => sl.position === input.position)!;
              (sl as any).result = { status: "OK", mark: 50120, markText: input.mark, rank: 1, qualify: "", attempts: input.attempts ?? [], bars: [], note: "" };
            }
            h.keepReason = "已有成绩";
            for (const t of detail.teams) t.keepReason = "已有成绩";
            changed("results", "meet", "event"); return null;
          }
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
          case "Marks.EventName": return `${args[0] === "混合" ? "混合" : args[0] === "女" ? "女子" : "男子"}${args[1]}${args[2] ? ` ${args[2]}` : ""}`;
          default: return error(`未模拟的方法：${method}`);
        }
      },
    };
  }, options);
}
