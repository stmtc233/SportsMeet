import type { Page } from "@playwright/test";

// Page tests exercise the UI with a deterministic mygo bridge. Business writes
// and actual HTTP MCP calls are verified against real SQLite by the Go tests.
export async function installRuntime(page: Page, options: { schedule?: boolean; staleSchedule?: boolean; scheduleErrors?: boolean; relay?: boolean; rejectTeamSave?: boolean; brokenEvent?: boolean; eventConfig?: "group" | "throw" | "count" | "score" | "teamCount"; emptyBoard?: boolean; conflicts?: boolean } = {}) {
  await page.addInitScript((options) => {
    const listeners = new Set<(change: unknown) => void>();
    let prefs = { theme: "system", separateUnits: true, balance: true, printNewPage: false, printSchedule: true, minRestMinutes: 0, checkInLeadMinutes: 20, printCheckInBreak: "round", printRecordBreak: "round", printResultsTime: false };
    let mcp = { enabled: false, running: false, port: 18765, address: "http://127.0.0.1:18765/mcp", error: "" };
    const meet = { id: "meet", name: "秋季运动会", startDate: "2026-11-12", endDate: "", location: "体育场", note: "", createdAt: 1, datesText: "2026-11-12" };
    const event = { id: "event", meetId: "meet", name: "男子100m 初中组", gender: "男", discipline: "100m", groupLabel: "初中组", kind: "track", date: "", laneCount: 8, attemptCount: 3, note: "", startMode: "lanes", distanceMode: "", relayLegCount: 0, participantMode: "athlete", teamSize: 0, scoreDecimals: 0, rankDirection: "", attemptMode: "" };
    const athletes = Array.from({ length: 24 }, (_, i) => ({ id: `a${i}`, meetId: "meet", name: `运动员${i + 1}`, gender: "男", bib: String(100 + i), studentNumber: "", unitId: "unit", className: "一班", grade: "", birthDate: "", note: "", createdAt: 1 }));
    const entries = athletes.map((a) => ({ athleteId: a.id, competitorId: a.id, name: a.name, gender: "男", bib: a.bib, unitId: "unit", unitName: "一班", seedMark: null, seedText: "", placed: false, warning: "" }));
    const rules = { timed: true, usesLanes: true, positionLabel: "道次", attemptLabel: "", startLabel: "分道", defaultPerHeat: 8, team: false, multipleAttempts: false, markLabel: "成绩", unit: "" };
    // What Go returns with a heat's times (service.HeatTimes): the own time, else the round's start.
    const shown = (at: string) => (at.startsWith("2026-11-12") ? at.slice(11) : at.replace("T", " "));
    const times = (roundStart: string, at: string, minutes: number | null) => {
      const effectiveAt = at || roundStart;
      return { scheduledAt: at, durationMinutes: minutes, effectiveAt, scheduleInherited: !at && !!roundStart, scheduledText: effectiveAt ? shown(effectiveAt) : "",
        durationText: minutes === null ? "" : `${minutes} 分钟`, endAt: "" };
    };
    // A heat's actual start (service.HeatStart) against its effective time; same-day minutes only.
    const minutesOf = (at: string) => Number(at.slice(11, 13)) * 60 + Number(at.slice(14, 16));
    const started = (effectiveAt: string, startedAt: string) => {
      const d = effectiveAt && startedAt ? minutesOf(startedAt) - minutesOf(effectiveAt) : null;
      return { startedAt, startedText: startedAt ? shown(startedAt) : "", startDeltaText: d === null ? "" : d === 0 ? "准时" : `${d > 0 ? "晚" : "早"} ${Math.abs(d)} 分钟` };
    };
    const unstarted = started("", "");
    const detail = { day: "2026-11-12", rules, event, teams: [] as any[], entries, rounds: [{ id: "round", stage: "final", ordinal: 1, name: "", label: "决赛", advance: null, scheduledAt: "", scheduledText: "", complete: false, standings: [], heats: [{ id: "heat", ordinal: 1, name: "", label: "第1组", ...times("", "2026-11-12T09:00", null), ...unstarted, resultsUpdatedText: "", base: "base", keepReason: "", slots: entries.slice(0, 8).map((a, i) => ({ position: i + 1, athleteId: a.athleteId, competitorId: a.competitorId, result: null })) }] }] };
    if (options.relay) {
      Object.assign(event, { relayLegCount: 4, participantMode: "team", kind: "relay", name: "男子4×100m", discipline: "4×100m", groupLabel: "" });
      rules.team = true;
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
      Object.assign(rules, { timed: false, usesLanes: false, positionLabel: "顺序", attemptLabel: "试投", multipleAttempts: true, startLabel: "", defaultPerHeat: 12 });
    }
    if (options.eventConfig === "count" || options.eventConfig === "teamCount") {
      Object.assign(event, { discipline: "跳绳", name: "男子跳绳", kind: "count", startMode: "", attemptMode: "best", rankDirection: "desc", attemptCount: 3 });
      Object.assign(rules, { timed: false, usesLanes: false, positionLabel: "顺序", multipleAttempts: true, attemptLabel: "尝试", markLabel: "次数", unit: "次", startLabel: "", defaultPerHeat: 12 });
      if (options.eventConfig === "teamCount") {
        Object.assign(event, { participantMode: "team", teamSize: 2, discipline: "集体跳绳", name: "男子集体跳绳" });
        rules.team = true;
        detail.entries.length = 0;
        detail.rounds[0]!.heats.length = 0;
      }
    }
    if (options.eventConfig === "score") {
      Object.assign(event, { discipline: "技巧评分", name: "男子技巧评分", kind: "score", startMode: "", attemptMode: "single", rankDirection: "desc", scoreDecimals: 2, attemptCount: 1 });
      Object.assign(rules, { timed: false, usesLanes: false, positionLabel: "顺序", multipleAttempts: false, markLabel: "分数", unit: "分", startLabel: "", defaultPerHeat: 12 });
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
        { id: "scored", ordinal: 2, name: "", label: "第2组", ...times("", "2026-11-12T09:07", null), ...started("2026-11-12T09:07", "2026-11-12T09:19"), resultsUpdatedText: "2026-11-12 09:20", base: "scored-base", keepReason: "已有成绩", slots: [] },
        { id: "later", ordinal: 3, name: "", label: "第3组", ...times("", "", null), ...unstarted, resultsUpdatedText: "", base: "later-base", keepReason: "", slots: [] },
      );
    }
    const actions = [["results.saved", "录入成绩", "mcp"], ["heats.scheduled", "批量排时间", "ui"], ["unit.created", "新建参赛单位", "ui"]] as const;
    const audit = Array.from({ length: 60 }, (_, i) => {
      const [action, actionText, source] = actions[i % 3]!;
      const minute = 59 - i;
      return { id: 60 - i, at: 1_800_000_000_000 - i * 60_000, atText: `2026-11-12 10:${String(minute).padStart(2, "0")}`, source, sourceText: source === "mcp" ? "MCP" : "界面", action, actionText, meetId: "meet", meetName: "秋季运动会", eventId: "", heatId: "", message: `记录${60 - i}` };
    });
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
      previewHash: "schedule-hash", errors: [] as string[], warnings: ["1 组不在赛事日期内"],
      changes: [
        { heatId: "heat", label: "第1组", before: "2026-11-12T09:00", after: "2026-11-12T23:58", beforeMinutes: null, afterMinutes: 3, beforeText: "2026-11-12 09:00", afterText: "2026-11-12 23:58 · 3 分钟" },
        { heatId: "later", label: "第3组", before: "", after: "2026-11-13T00:01", beforeMinutes: null, afterMinutes: 3, beforeText: "未安排", afterText: "2026-11-13 00:01 · 3 分钟" },
      ],
      skipped: [{ heatId: "scored", label: "第2组", reason: "已有成绩" }],
      conflicts: [] as any[],
    };
    // The schedule page's timetable (service.Timetable), built from the event
    // detail plus a 跳远 round at 09:00 and an unscheduled 跳高 round. With
    // options.conflicts, 运动员1 is in the 100m 第1组 and the 跳远 heat.
    (globalThis as any).__heatEdits = [];
    const ref = (eventId: string, eventName: string, roundId: string, heatId: string, heatLabel: string, timeText: string) => ({ eventId, eventName, roundId, roundLabel: "决赛", heatId, heatLabel, timeText, text: `${timeText} ${eventName} 决赛 ${heatLabel}` });
    const conflicts = () => options.conflicts ? [{ athleteId: "a0", athleteName: "运动员1", overlap: true, gapMinutes: -5, text: "时间重叠",
      first: ref("event", event.name, "round", "heat", "第1组", "2026-11-12 09:00"), second: ref("jump", "男子跳远", "jump-round", "jump-heat", "第1组", "2026-11-12 09:00") }] : [];
    const heatConflicts = (heatId: string) => conflicts().filter((c) => c.first.heatId === heatId || c.second.heatId === heatId)
      .map((c) => ({ athleteId: c.athleteId, athleteName: c.athleteName, text: c.text, other: c.first.heatId === heatId ? c.second : c.first }));
    const timetableHeat = (h: any) => ({ id: h.id, label: h.label, name: h.name, base: h.base, scheduledAt: h.scheduledAt, durationMinutes: h.durationMinutes, effectiveAt: h.effectiveAt,
      scheduleInherited: h.scheduleInherited, scheduledText: h.scheduledText, durationText: h.durationText, endAt: "", endText: "", ...started(h.effectiveAt, h.startedAt ?? ""), keepReason: h.keepReason, people: h.slots.length, conflicts: heatConflicts(h.id) });
    const timetable = () => {
      const r = detail.rounds[0]!;
      const heats = r.heats.map(timetableHeat);
      const startAt = heats.map((h) => h.effectiveAt).filter(Boolean).sort()[0] ?? "";
      const sprint = { eventId: event.id, eventName: event.name, roundId: r.id, roundLabel: r.label, day: startAt.slice(0, 10), scheduledAt: r.scheduledAt, scheduledText: r.scheduledText,
        startAt, startText: startAt ? shown(startAt) : "", heatCount: heats.length, people: heats.reduce((n, h) => n + h.people, 0), peopleEntered: false, heats };
      const jumpHeat = { id: "jump-heat", label: "第1组", name: "", base: "jump-base", scheduledAt: "", durationMinutes: 30, effectiveAt: "2026-11-12T09:00", scheduleInherited: true, scheduledText: "09:00",
        durationText: "30 分钟", endAt: "2026-11-12T09:30", endText: "09:30", ...unstarted, keepReason: "", people: 1, conflicts: heatConflicts("jump-heat") };
      const jump = { eventId: "jump", eventName: "男子跳远", roundId: "jump-round", roundLabel: "决赛", day: "2026-11-12", scheduledAt: "2026-11-12T09:00", scheduledText: "09:00",
        startAt: "2026-11-12T09:00", startText: "09:00", heatCount: 1, people: 1, peopleEntered: false, heats: [jumpHeat] };
      const high = { eventId: "high", eventName: "男子跳高", roundId: "high-round", roundLabel: "决赛", day: "", scheduledAt: "", scheduledText: "", startAt: "", startText: "", heatCount: 0, people: 12, peopleEntered: true, heats: [] };
      const day = [sprint, jump].filter((x) => x.day).sort((a, b) => a.startAt.localeCompare(b.startAt));
      return { minRestMinutes: prefs.minRestMinutes, days: [{ day: "2026-11-12", rounds: day }], unscheduled: sprint.day ? [high] : [sprint, high], conflicts: conflicts() };
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
          case "Settings.AuditLog": {
            const q = args[0];
            const all = audit.filter((r) => (!q.meetId || r.meetId === q.meetId) && (!q.source || r.source === q.source));
            const start = q.after ? all.findIndex((r) => r.id < q.after.id) : 0;
            const rows = start < 0 ? [] : all.slice(start, start + q.limit);
            const last = rows[rows.length - 1];
            return { rows, next: last && all.indexOf(last) < all.length - 1 ? { at: last.at, id: last.id } : null };
          }
          case "Settings.CopyToken": case "Settings.CopyConnection": return null;
          case "App.Info": return { version: "0.1.0" };
          case "Meets.List": return [meet];
          case "Meets.Get": return meet;
          case "Units.List": return [{ id: "unit", meetId: "meet", name: "一班", shortName: "" }];
          case "Athletes.List": return athletes.map((athlete) => ({ athlete, unitName: "一班", entries: 1, placed: true }));
          case "Events.List": return [{ event, autoName: true, prelim: false, advance: null, entries: 24, heats: 1, placed: 8, results: 0, timeText: "09:00" }];
          case "Events.Presets": return presets;
          case "Events.Rules": {
            const e = args[0];
            const timed = e.kind === "track" || e.kind === "relay";
            const usesLanes = timed && e.startMode !== "group";
            return { timed, usesLanes, team: e.kind === "relay" || e.participantMode === "team", multipleAttempts: e.kind === "distance" || ((e.kind === "count" || e.kind === "score") && e.attemptMode && e.attemptMode !== "single"), markLabel: e.kind === "count" ? "次数" : e.kind === "score" ? "分数" : "成绩", positionLabel: usesLanes ? "道次" : "顺序", attemptLabel: e.kind === "distance" ? (e.distanceMode === "throw" ? "试投" : "试跳") : "尝试", defaultPerHeat: usesLanes ? e.laneCount : 12 };
          }
          case "Events.TeamWarnings": {
            const n = args[1].length;
            return event.relayLegCount > 0 && event.relayLegCount !== n ? ["棒数与项目建议不符"] : event.teamSize > 0 && event.teamSize !== n ? ["人数与项目建议不符"] : [];
          }
          case "Events.TeamSizeWarning": return args[0].relayLegCount > 0 && args[0].relayLegCount !== args[1] ? "棒数与项目建议不符" : args[0].teamSize > 0 && args[0].teamSize !== args[1] ? "人数与项目建议不符" : "";
          case "Events.Save": {
            (globalThis as any).__eventCalls.push(structuredClone(args));
            Object.assign(event, args[2], { name: args[2].name || `${args[2].gender === "混合" ? "混合" : args[2].gender === "女" ? "女子" : "男子"}${args[2].discipline}` });
            changed("events", "meet", "event"); return structuredClone(event);
          }
          case "Events.Get": return options.brokenEvent ? {} : structuredClone(detail);
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
            return [{ slots: detail.entries.map((e, i) => ({ position: (rules.usesLanes ? 4 : 1) + i, competitorId: e.competitorId })) }];
          case "Arrange.Save": {
            (globalThis as any).__relayCalls.push({ method, args });
            detail.rounds[0]!.heats = args[0].rounds[0].added.map((h: any, i: number) => ({ id: `heat${i}`, ordinal: i + 1, name: "", label: `第${i + 1}组`, base: `saved${i}`, keepReason: "", ...times(detail.rounds[0]!.scheduledAt, "", null), ...unstarted, resultsUpdatedText: "", slots: h.slots.map((sl: any) => ({ ...sl, athleteId: rules.team ? "" : sl.competitorId, teamId: rules.team ? sl.competitorId : "", result: null })) }));
            for (const t of detail.teams) t.placed = true;
            changed("heats", "meet", "event"); return structuredClone(detail);
          }
          case "Arrange.HeatPlan": {
            const [roundStart, at, text] = args as [string, string, string];
            const minutes = text.trim() === "" ? null : Number(text);
            if (minutes !== null && !(Number.isInteger(minutes) && minutes >= 1 && minutes <= 1440)) return error("预计用时须为 1–1440 分钟");
            return times(roundStart, at, minutes);
          }
          case "Arrange.ScheduleWarning": return args[1] && !args[1].startsWith("2026-11-12") ? "时间不在赛事日期内" : "";
          case "Arrange.SetRoundStart": {
            const { roundId, scheduledAt } = args[0];
            (globalThis as any).__roundStarts = [...((globalThis as any).__roundStarts ?? []), scheduledAt];
            const round = detail.rounds.find((r) => r.id === roundId)!;
            Object.assign(round, { scheduledAt, scheduledText: scheduledAt ? shown(scheduledAt) : "" });
            round.heats = round.heats.map((h: any) => ({ ...h, ...times(scheduledAt, h.scheduledAt, h.durationMinutes) }));
            changed("heats", "meet", "event");
            return { warnings: scheduledAt && !scheduledAt.startsWith("2026-11-12") ? ["时间不在赛事日期内"] : [] };
          }
          case "Marks.NormalizeEvent": return args[0].kind === "score" ? Number(args[1]).toFixed(args[0].scoreDecimals ?? 0) : args[1];
          case "Marks.Normalize": return args[1];
          case "Results.Save": {
            (globalThis as any).__relayCalls.push({ method, args });
            const h = detail.rounds[0]!.heats.find((h) => h.id === args[0])!;
            for (const input of args[1]) {
              const sl = h.slots.find((sl) => sl.position === input.position)!;
              (sl as any).result = { status: "OK", mark: 50120, markText: event.kind === "score" ? Number(input.mark).toFixed(event.scoreDecimals) : event.kind === "count" ? String(Math.max(...(input.attempts ?? []).map(Number).filter(Number.isFinite))) : input.mark, rank: 1, qualify: "", attempts: input.attempts ?? [], bars: [], note: "" };
            }
            h.keepReason = "已有成绩";
            (h as any).resultsUpdatedText = "2026-11-12 09:31";
            if (!h.startedAt) Object.assign(h, started(h.effectiveAt, "2026-11-12T09:12"));
            for (const t of detail.teams) t.keepReason = "已有成绩";
            changed("results", "meet", "event"); return null;
          }
          case "Results.SetStarted": {
            const { heatId, startedAt } = args[0];
            (globalThis as any).__startedCalls = [...((globalThis as any).__startedCalls ?? []), structuredClone(args[0])];
            const h = detail.rounds[0]!.heats.find((h) => h.id === heatId)!;
            Object.assign(h, started(h.effectiveAt, startedAt));
            changed("heats", "meet", "event");
            return { heatId, ...started(h.effectiveAt, startedAt) };
          }
          case "Arrange.PreviewSchedule": {
            (globalThis as any).__scheduleCalls.preview++;
            (globalThis as any).__scheduleCalls.request = structuredClone(args[0]);
            if (options.scheduleErrors) return { previewHash: "", changes: [], skipped: [], warnings: [], conflicts: [], errors: ["间隔须为 1–1440 分钟"] };
            const p = structuredClone(schedulePreview);
            if (args[0].rounds) {
              p.changes = p.changes.map((c: any) => ({ ...c, eventId: event.id, roundId: "round", round: `${event.name} 决赛` }));
              p.skipped = p.skipped.map((c: any) => ({ ...c, eventId: event.id, roundId: "round", round: `${event.name} 决赛` }));
              p.conflicts = conflicts();
              if (p.conflicts.length) p.warnings = [...p.warnings, `排完后有 ${p.conflicts.length} 处时间冲突`];
            }
            return p;
          }
          case "Arrange.MeetSchedule": return timetable();
          case "Arrange.DraftConflicts": {
            // The draft's own heats by key; 运动员1 placed in a heat at 09:00 meets the 跳远 heat.
            (globalThis as any).__draftConflictCalls = ((globalThis as any).__draftConflictCalls ?? 0) + 1;
            if (!options.conflicts) return [];
            const r = detail.rounds[0]!;
            return args[0].heats.filter((h: any) => h.competitors.includes("a0") && (h.scheduledAt || r.scheduledAt || (r.heats.find((x) => x.id === h.key)?.effectiveAt ?? "")) === "2026-11-12T09:00")
              .map((h: any) => ({ heatId: h.key, conflicts: heatConflicts("heat") }));
          }
          case "Arrange.EditHeat": {
            const input = args[0];
            (globalThis as any).__heatEdits.push(structuredClone(input));
            const r = detail.rounds[0]!;
            const heat = r.heats.find((h) => h.id === input.heatId)!;
            if (heat.base !== input.base) return error("E_STALE");
            Object.assign(heat, times(r.scheduledAt, input.scheduledAt, input.durationMinutes), { name: input.name, base: heat.base + "-edited" });
            changed("heats", "meet", "event"); return { base: heat.base, warnings: [], conflicts: [] };
          }
          case "Arrange.Schedule":
            (globalThis as any).__scheduleCalls.save++;
            if (staleSchedule) { staleSchedule = false; return error("E_STALE"); }
            for (const change of schedulePreview.changes) {
              const heat = detail.rounds[0]!.heats.find((h) => h.id === change.heatId)!;
              Object.assign(heat, times(detail.rounds[0]!.scheduledAt, change.after, change.afterMinutes));
              heat.base += "-scheduled";
            }
            changed("heats", "meet", "event");
            return structuredClone(schedulePreview);
          case "Reports.Targets": return [{ id: event.id, name: event.name }];
          case "Reports.CheckInTargets": return {
            days: ["2026-11-12", "2026-11-13"],
            events: [
              { id: event.id, name: event.name, rounds: [{ id: "round", label: "决赛", heatCount: detail.rounds[0]!.heats.length, days: ["2026-11-12"] }] },
              { id: "jump", name: "男子跳远", rounds: [{ id: "jump-round", label: "决赛", heatCount: 1, days: ["2026-11-13"] }] },
            ],
          };
          case "Reports.RenderCheckIn":
            (globalThis as any).__checkInCalls = [...((globalThis as any).__checkInCalls ?? []), structuredClone(args[0])];
            return `<!DOCTYPE html><html><body><h1>检录单</h1><p>${args[0].day || "全部日期"} · ${args[0].roundIds.join(",") || "全部轮次"}</p></body></html>`;
          case "Reports.PrintCheckIn":
            (globalThis as any).__checkInPrints = [...((globalThis as any).__checkInPrints ?? []), structuredClone(args[0])];
            return null;
          case "Reports.RenderRecordSheet":
            return `<!DOCTYPE html><html><body><h1>成绩记录表</h1><p>${args[0].day || "全部日期"} · ${args[0].roundIds.join(",") || "全部轮次"}</p></body></html>`;
          case "Reports.PrintRecordSheet":
            (globalThis as any).__recordPrints = [...((globalThis as any).__recordPrints ?? []), structuredClone(args[0])];
            return null;
          case "Combined.List": return [];
          case "Reports.Render": return "<!DOCTYPE html><html><body><h1>秋季运动会</h1><p>秩序册</p></body></html>";
          case "Marks.EventName": return `${args[0] === "混合" ? "混合" : args[0] === "女" ? "女子" : "男子"}${args[1]}${args[2] ? ` ${args[2]}` : ""}`;
          default: return error(`未模拟的方法：${method}`);
        }
      },
    };
  }, options);
}
