import type { Page } from "@playwright/test";
import { installRuntime } from "./runtime";

export async function installCombinedRuntime(page: Page, rejectCreate = false) {
  await installRuntime(page);
  await page.addInitScript((rejectCreate) => {
    const bridge = (globalThis as any).mygo;
    const baseCall = bridge.call;
    const baseOn = bridge.on;
    const listeners = new Set<(change: any) => void>();
    bridge.on = (name: string, fn: any) => { listeners.add(fn); const off = baseOn(name, fn); return () => { listeners.delete(fn); off(); }; };
    const changed = () => { for (const fn of listeners) fn({ kind: "combined", meetId: "meet", eventId: "" }); };
    const athletes = Array.from({ length: 3 }, (_, i) => ({ id: `a${i}`, meetId: "meet", name: `运动员${i + 1}`, gender: "男", bib: String(100 + i), unitId: "unit", studentNumber: "", className: "", grade: "", birthDate: "", note: "" }));
    let detail: any;
    const children: Record<string, any> = {};
    let pendingRules: any;
    const clone = (v: any) => structuredClone(v);
    const parseRule = (event: any, rule: any) => ({ rule: { bands: [], invalidPolicies: rule.invalidPolicies }, rows: rule.table.trim().split("\n").map((line: string) => { const [markText, points] = line.trim().split(/[\s,]+/); return { markText, points: Number(points) }; }) });
    const recompute = (rules?: any[]) => {
      if (!detail) return [];
      const rows = detail.athleteIds.map((id: string) => {
        const a = athletes.find((a) => a.id === id)!;
        const cells = detail.components.map((c: any) => {
          const child = children[c.event.id];
          const result = child.rounds[0].heats.flatMap((h: any) => h.slots).find((s: any) => s.athleteId === id)?.result;
          if (!result) return { eventId: c.event.id, markText: "", points: null, pending: true, excluded: false };
          const rule = rules?.find((r: any) => r.eventId === c.event.id)?.rule ?? c.input;
          const band = parseRule(c.event, rule).rows.find((b: any) => c.event.kind === "track" ? Number(result.markText) <= Number(b.markText) : Number(result.markText) >= Number(b.markText));
          return { eventId: c.event.id, markText: result.markText, points: band?.points ?? 0, pending: false, excluded: false };
        });
        return { athleteId: id, name: a.name, bib: a.bib, unitName: "一班", cells, total: cells.reduce((n: number, c: any) => n + (c.points ?? 0), 0), pending: cells.some((c: any) => c.pending), excluded: false, rank: null };
      }).sort((a: any, b: any) => b.total - a.total);
      rows.forEach((r: any, i: number) => r.rank = i + 1);
      if (!rules) { detail.rows = rows; detail.complete = rows.length > 0 && rows.every((r: any) => !r.pending); }
      return rows;
    };
    (globalThis as any).__combinedCalls = [];
    bridge.call = async (method: string, ...args: any[]) => {
      if (method.startsWith("Combined.")) (globalThis as any).__combinedCalls.push({ method, args: clone(args) });
      switch (method) {
        case "Combined.List": return detail ? [{ combined: detail.combined, components: detail.components.length, entries: detail.athleteIds.length, results: detail.results }] : [];
        case "Combined.Save": {
          if (rejectCreate) { const err = new Error("计分表第 1 行成绩有误"); err.name = "CallError"; throw err; }
          const input = args[2];
          detail = { combined: { id: "combined", meetId: "meet", name: input.name, gender: input.gender, groupLabel: input.groupLabel, note: "", ruleVersion: 1 }, athleteIds: [], athletes, rows: [], components: [], complete: false, hasPlacements: false, heats: 0, results: 0 };
          detail.components = input.components.map((p: any, i: number) => {
            const event = { id: `component${i}`, meetId: "meet", combinedId: "combined", ...p.event, name: `${input.name} · ${p.event.discipline}` };
            const timed = event.kind === "track";
            children[event.id] = { event, teams: [], entries: [], rules: { timed, team: false, usesLanes: timed, multipleAttempts: event.kind === "distance", positionLabel: timed ? "道次" : "顺序", attemptLabel: "试跳", markLabel: "成绩", defaultPerHeat: timed ? 8 : 12 }, rounds: [{ id: `round${i}`, stage: "final", ordinal: 1, label: "决赛", advance: null, complete: false, standings: [], heats: [] }] };
            return { event, ordinal: i + 1, input: p.rule, rule: parseRule(event, p.rule).rule };
          });
          changed(); return clone(detail.combined);
        }
        case "Combined.ParseRule": return parseRule(args[0], args[1]);
        case "Combined.Get": recompute(); return clone(detail);
        case "Combined.Entries": {
          detail.athleteIds = args[1];
          for (const child of Object.values(children)) child.entries = args[1].map((id: string) => { const a = athletes.find((a) => a.id === id)!; return { athleteId: id, competitorId: id, name: a.name, gender: a.gender, bib: a.bib, unitId: "unit", unitName: "一班", seedText: "", seedMark: null, placed: false, genderMismatch: false }; });
          recompute(); changed(); return null;
        }
        case "Combined.PreviewRules": recompute(); pendingRules = args[0].rules; return { previewHash: "rule-preview", ruleVersion: detail.combined.ruleVersion + 1, before: clone(detail.rows), after: clone(recompute(pendingRules)), changed: true };
        case "Combined.ApplyRules":
          for (const r of pendingRules) detail.components.find((c: any) => c.event.id === r.eventId).input = clone(r.rule);
          detail.combined.ruleVersion++; recompute(); changed(); return { previewHash: "rule-preview", changed: true };
        case "Events.Get": if (children[args[0]]) return clone(children[args[0]]); break;
        case "Events.List": return [...await baseCall(method, ...args), ...Object.values(children).map((c: any) => ({ event: c.event, entries: c.entries.length, heats: c.rounds[0].heats.length, results: 0, autoName: false, prelim: false, placed: 0 }))];
        case "Arrange.Plan": if (children[args[0].eventId]) return [{ slots: children[args[0].eventId].entries.map((e: any, i: number) => ({ competitorId: e.competitorId, position: i + 1 })) }]; break;
        case "Arrange.Save": if (children[args[0].eventId]) {
          const child = children[args[0].eventId];
          child.rounds[0].heats = args[0].rounds[0].added.map((h: any) => ({ id: `heat-${child.event.id}`, label: "第1组", ordinal: 1, name: "", base: "saved", keepReason: "", scheduledAt: "", slots: h.slots.map((s: any) => ({ ...s, athleteId: s.competitorId, result: null })) }));
          detail.hasPlacements = true; detail.heats++; changed(); return clone(child);
        } break;
        case "Results.Save": {
          const child: any = Object.values(children).find((c: any) => c.rounds[0].heats.some((h: any) => h.id === args[0]));
          if (!child) break;
          const heat = child.rounds[0].heats[0];
          for (const input of args[1]) {
            const markText = input.mark || Math.max(...input.attempts.map(Number).filter(Number.isFinite)).toFixed(2);
            const result = { status: "OK", mark: Number(markText) * 1000, markText, rank: 1, qualify: "", attempts: input.attempts ?? [], bars: [], note: "" };
            heat.slots.find((s: any) => s.position === input.position).result = result;
          }
          detail.results += args[1].length; heat.keepReason = "已有成绩"; recompute(); changed(); return null;
        }
      }
      return baseCall(method, ...args);
    };
  }, rejectCreate);
}
