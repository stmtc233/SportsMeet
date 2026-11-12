// The arrange page's local draft (docs/agents/arrange.md): an immutable
// value changed by the pure functions below, saved in one go. Each loaded
// heat keeps the fingerprint (`base`) it came with; saving submits only the
// heats that differ from what was loaded, and a data change merges heat by
// heat on that fingerprint. Arrangement rules (seeding, ordinals, what a
// result locks) stay in Go; this file only moves athletes between slots.
import type { BoardSave, EventDetail, HeatRef, HeatSave, PlannedHeat, RoundSave } from "../../mygo";

export interface DraftSlot {
  position: number;
  athleteId: string;
}

export interface DraftHeat {
  /** The heat's id, or a local key for a heat not saved yet. */
  key: string;
  /** "" for a heat not saved yet. */
  id: string;
  base: string;
  label: string;
  name: string;
  scheduledAt: string;
  slots: DraftSlot[];
  /** Why the heat is locked ("" = it is not). */
  keepReason: string;
}

export interface DraftRound {
  roundId: string;
  heats: DraftHeat[];
  deleted: HeatRef[];
}

export interface ArrangeDraft {
  rounds: DraftRound[];
  /** Locked heats the user unlocked. */
  unlocked: string[];
  /** 「全部重置」: heats with results may be deleted. */
  reset: boolean;
}

export type Edit = { draft: ArrangeDraft; error?: undefined } | { draft?: undefined; error: string };

let counter = 0;
export function newHeatKey(): string {
  counter += 1;
  return `new-${counter}`;
}

export function fromDetail(detail: EventDetail): ArrangeDraft {
  return {
    unlocked: [],
    reset: false,
    rounds: detail.rounds.map((r) => ({
      roundId: r.id,
      deleted: [],
      heats: r.heats.map((h) => ({
        key: h.id,
        id: h.id,
        base: h.base,
        label: h.label,
        name: h.name,
        scheduledAt: h.scheduledAt,
        keepReason: h.keepReason,
        slots: h.slots.map((s) => ({ position: s.position, athleteId: s.athleteId })),
      })),
    })),
  };
}

function sameSlots(a: DraftSlot[], b: DraftSlot[]): boolean {
  return a.length === b.length && a.every((s, i) => s.position === b[i]!.position && s.athleteId === b[i]!.athleteId);
}

export function heatChanged(h: DraftHeat, loaded: DraftHeat | undefined): boolean {
  return !loaded || h.name !== loaded.name || h.scheduledAt !== loaded.scheduledAt || !sameSlots(h.slots, loaded.slots);
}

export function isLocked(draft: ArrangeDraft, heat: DraftHeat): boolean {
  return heat.keepReason !== "" && !draft.reset && !draft.unlocked.includes(heat.key);
}

function round(draft: ArrangeDraft, roundId: string): DraftRound | undefined {
  return draft.rounds.find((r) => r.roundId === roundId);
}

function withRound(draft: ArrangeDraft, roundId: string, change: (r: DraftRound) => DraftRound): ArrangeDraft {
  return { ...draft, rounds: draft.rounds.map((r) => (r.roundId === roundId ? change(r) : r)) };
}

function sorted(slots: DraftSlot[]): DraftSlot[] {
  return [...slots].sort((a, b) => a.position - b.position);
}

// Field events have an order, not lanes: positions stay 1…n without gaps.
function tidy(slots: DraftSlot[], timed: boolean): DraftSlot[] {
  const list = sorted(slots);
  return timed ? list : list.map((s, i) => ({ ...s, position: i + 1 }));
}

/** The athletes placed in a round, with the heat holding each. */
export function placedIn(r: DraftRound | undefined): Map<string, DraftHeat> {
  const out = new Map<string, DraftHeat>();
  for (const h of r?.heats ?? []) for (const s of h.slots) out.set(s.athleteId, h);
  return out;
}

const lockedText = (h: DraftHeat) => `「${h.label}」${h.keepReason}，请先解锁位置`;

/**
 * Puts an athlete at a position. Coming from another slot of the round,
 * the two swap when the position is taken; coming from the list, whoever
 * is there goes back to the list.
 */
export function place(draft: ArrangeDraft, roundId: string, heatKey: string, position: number, athleteId: string, timed: boolean): Edit {
  const r = round(draft, roundId);
  const target = r?.heats.find((h) => h.key === heatKey);
  if (!r || !target) return { error: "该组已不存在" };
  if (isLocked(draft, target)) return { error: lockedText(target) };
  const source = r.heats.find((h) => h.slots.some((s) => s.athleteId === athleteId));
  if (source && isLocked(draft, source)) return { error: lockedText(source) };
  const from = source?.slots.find((s) => s.athleteId === athleteId)?.position;
  const occupant = target.slots.find((s) => s.position === position)?.athleteId;
  if (occupant === athleteId) return { draft };

  const heats = r.heats.map((h) => {
    if (h !== target && h !== source) return h;
    let slots = h.slots.filter((s) => s.athleteId !== athleteId && s.athleteId !== occupant);
    if (h === target) slots = [...slots, { position, athleteId }];
    if (h === source && occupant && from !== undefined) slots = [...slots, { position: from, athleteId: occupant }];
    return { ...h, slots: tidy(slots, timed) };
  });
  return { draft: withRound(draft, roundId, (old) => ({ ...old, heats })) };
}

/** Takes an athlete out of the round, back to the list. */
export function unplace(draft: ArrangeDraft, roundId: string, athleteId: string, timed: boolean): Edit {
  const r = round(draft, roundId);
  const source = r?.heats.find((h) => h.slots.some((s) => s.athleteId === athleteId));
  if (!r || !source) return { draft };
  if (isLocked(draft, source)) return { error: lockedText(source) };
  const heats = r.heats.map((h) => (h === source ? { ...h, slots: tidy(h.slots.filter((s) => s.athleteId !== athleteId), timed) } : h));
  return { draft: withRound(draft, roundId, (old) => ({ ...old, heats })) };
}

function blankHeat(slots: DraftSlot[] = []): DraftHeat {
  return { key: newHeatKey(), id: "", base: "", label: "", name: "", scheduledAt: "", keepReason: "", slots };
}

export function addHeat(draft: ArrangeDraft, roundId: string): ArrangeDraft {
  return withRound(draft, roundId, (r) => ({ ...r, heats: [...r.heats, blankHeat()] }));
}

/** Appends heats planned by Go's arrange as they are. */
export function appendPlanned(draft: ArrangeDraft, roundId: string, planned: PlannedHeat[]): ArrangeDraft {
  const heats = planned.map((p) => blankHeat(sorted(p.slots.map((s) => ({ position: s.position, athleteId: s.athleteId })))));
  return withRound(draft, roundId, (r) => ({ ...r, heats: [...r.heats, ...heats] }));
}

export function deleteHeat(draft: ArrangeDraft, roundId: string, key: string): Edit {
  const heat = round(draft, roundId)?.heats.find((h) => h.key === key);
  if (!heat) return { draft };
  if (heat.keepReason !== "" && !draft.reset) return { error: `「${heat.label}」${heat.keepReason}，不能删除` };
  return {
    draft: withRound(draft, roundId, (r) => ({
      ...r,
      heats: r.heats.filter((h) => h.key !== key),
      deleted: heat.id ? [...r.deleted, { id: heat.id, base: heat.base }] : r.deleted,
    })),
  };
}

export function moveHeat(draft: ArrangeDraft, roundId: string, key: string, delta: -1 | 1): ArrangeDraft {
  return withRound(draft, roundId, (r) => {
    const i = r.heats.findIndex((h) => h.key === key);
    const j = i + delta;
    if (i < 0 || j < 0 || j >= r.heats.length) return r;
    const heats = [...r.heats];
    [heats[i], heats[j]] = [heats[j]!, heats[i]!];
    return { ...r, heats };
  });
}

export function editHeat(draft: ArrangeDraft, roundId: string, key: string, patch: { name: string; scheduledAt: string }): ArrangeDraft {
  return withRound(draft, roundId, (r) => ({ ...r, heats: r.heats.map((h) => (h.key === key ? { ...h, ...patch } : h)) }));
}

export function unlock(draft: ArrangeDraft, key: string): ArrangeDraft {
  return draft.unlocked.includes(key) ? draft : { ...draft, unlocked: [...draft.unlocked, key] };
}

/** 「全部重置」: every heat of the event goes, results included. */
export function resetAll(draft: ArrangeDraft): ArrangeDraft {
  return {
    unlocked: [],
    reset: true,
    rounds: draft.rounds.map((r) => ({
      roundId: r.roundId,
      heats: [],
      deleted: [...r.deleted, ...r.heats.filter((h) => h.id).map((h) => ({ id: h.id, base: h.base }))],
    })),
  };
}

function toSave(h: DraftHeat): HeatSave {
  return { id: h.id, key: h.id ? "" : h.key, base: h.base, name: h.name, scheduledAt: h.scheduledAt, slots: h.slots };
}

/** What saving would submit, and how many heats it touches. */
export function diff(draft: ArrangeDraft, loaded: ArrangeDraft, eventId: string): { save: BoardSave; count: number } {
  const rounds: RoundSave[] = [];
  let count = 0;
  for (const r of draft.rounds) {
    const was = round(loaded, r.roundId);
    const byKey = new Map(was?.heats.map((h) => [h.key, h]));
    const changed = r.heats.filter((h) => h.id && heatChanged(h, byKey.get(h.key))).map(toSave);
    const added = r.heats.filter((h) => !h.id).map(toSave);
    const order = r.heats.map((h) => h.key);
    const kept = (was?.heats ?? []).map((h) => h.key).filter((k) => !r.deleted.some((d) => d.id === k));
    const moved = order.filter((k) => byKey.has(k)).join() !== kept.join();
    const n = changed.length + added.length + r.deleted.length;
    if (n === 0 && !moved) continue;
    count += n + (moved && n === 0 ? 1 : 0);
    rounds.push({ roundId: r.roundId, order, changed, added, deleted: r.deleted });
  }
  return { save: { eventId, rounds, unlocked: draft.unlocked, reset: draft.reset }, count };
}

/**
 * Brings fresh data into a draft. A heat changed elsewhere (its
 * fingerprint differs from the one loaded) replaces the local version;
 * every other local change stays. `dropped` says a local change was lost.
 */
export function merge(draft: ArrangeDraft, loaded: ArrangeDraft, fresh: ArrangeDraft): { draft: ArrangeDraft; dropped: boolean } {
  let dropped = false;
  const rounds = fresh.rounds.map((f): DraftRound => {
    const mine = round(draft, f.roundId);
    const was = round(loaded, f.roundId);
    if (!mine || !was) return f;
    const wasBy = new Map(was.heats.map((h) => [h.key, h]));
    const freshBy = new Map(f.heats.map((h) => [h.key, h]));
    const mineBy = new Map(mine.heats.map((h) => [h.key, h]));

    const deleted = mine.deleted.filter((d) => {
      const now = freshBy.get(d.id);
      if (!now) return false; // gone elsewhere too
      if (draft.reset) return true;
      if (now.base === d.base) return true;
      dropped = true;
      return false;
    }).map((d) => (draft.reset ? { id: d.id, base: freshBy.get(d.id)!.base } : d));
    if (draft.reset) {
      // A reset deletes whatever is there now.
      const all = f.heats.map((h) => ({ id: h.id, base: h.base }));
      return { roundId: f.roundId, heats: mine.heats.filter((h) => !h.id), deleted: all };
    }
    const gone = new Set(deleted.map((d) => d.id));

    const fromFresh = new Set<string>();
    const resolve = (key: string): DraftHeat | undefined => {
      const now = freshBy.get(key);
      if (!now || gone.has(key)) return undefined;
      const local = mineBy.get(key);
      const before = wasBy.get(key);
      if (local && before && heatChanged(local, before)) {
        if (before.base === now.base) return { ...local, keepReason: now.keepReason, label: now.label };
        if (heatChanged(local, now)) dropped = true; // the same content means this was our own save
      }
      fromFresh.add(key);
      return now;
    };

    const localOrder = mine.heats.filter((h) => h.id).map((h) => h.key);
    const loadedOrder = was.heats.map((h) => h.key).filter((k) => mineBy.has(k));
    const reordered = localOrder.join() !== loadedOrder.join();
    let heats: DraftHeat[];
    if (reordered) {
      heats = mine.heats.flatMap((h) => (h.id ? resolve(h.key) ?? [] : h));
      for (const h of f.heats) if (!mineBy.has(h.key) && !gone.has(h.key)) heats.push(resolve(h.key)!);
    } else {
      heats = [...f.heats.flatMap((h) => resolve(h.key) ?? []), ...mine.heats.filter((h) => !h.id)];
    }
    for (const h of mine.heats) if (h.id && !freshBy.has(h.key) && heatChanged(h, wasBy.get(h.key))) dropped = true;

    // One slot per athlete: heats taken fresh win over local ones.
    const taken = new Set<string>();
    for (const h of heats) if (fromFresh.has(h.key)) for (const s of h.slots) taken.add(s.athleteId);
    heats = heats.map((h) => {
      if (fromFresh.has(h.key)) return h;
      const slots = h.slots.filter((s) => !taken.has(s.athleteId));
      if (slots.length !== h.slots.length) dropped = true;
      return slots.length === h.slots.length ? h : { ...h, slots };
    });
    return { roundId: f.roundId, heats, deleted };
  });
  const keys = new Set(rounds.flatMap((r) => r.heats.map((h) => h.key)));
  return { draft: { rounds, reset: draft.reset, unlocked: draft.unlocked.filter((k) => keys.has(k)) }, dropped };
}

export interface History {
  past: ArrangeDraft[];
  present: ArrangeDraft;
  future: ArrangeDraft[];
}

const historyLimit = 100;

export function pushHistory(h: History, next: ArrangeDraft): History {
  if (next === h.present) return h;
  return { past: [...h.past, h.present].slice(-historyLimit), present: next, future: [] };
}

export function undo(h: History): History {
  const prev = h.past[h.past.length - 1];
  return prev ? { past: h.past.slice(0, -1), present: prev, future: [h.present, ...h.future] } : h;
}

export function redo(h: History): History {
  const next = h.future[0];
  return next ? { past: [...h.past, h.present], present: next, future: h.future.slice(1) } : h;
}
