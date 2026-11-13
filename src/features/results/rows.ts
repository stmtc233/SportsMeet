// What is typed for a heat's results, and how it survives a reload
// (docs/agents/frontend.md「成绩页」). Pure, so it is tested without a page.
import type { SlotView } from "../../mygo";

/** One position's inputs as typed. */
export interface Row {
  competitorId: string; // who the inputs are for; not sent
  status: string;
  mark: string;
  attempts: string[];
  bars: Record<string, string>; // height text → trials
}

/** Rows by position. */
export type Rows = Record<number, Row>;

export function toRow(slot: SlotView, attemptCount: number): Row {
  const r = slot.result;
  const attempts = Array.from({ length: Math.max(attemptCount, r?.attempts.length ?? 0) }, (_, i) => r?.attempts[i] ?? "");
  const derived = r && (r.status === "OK" || (r.status === "NM" && (r.attempts.length > 0 || r.bars.length > 0)));
  return { competitorId: slot.competitorId, status: r && !derived ? r.status : "", mark: r?.markText ?? "", attempts, bars: Object.fromEntries((r?.bars ?? []).map((b) => [b.heightText, b.trials])) };
}

export function toRows(slots: SlotView[], attemptCount: number): Rows {
  return Object.fromEntries(slots.map((s) => [s.position, toRow(s, attemptCount)]));
}

export function sameRow(a: Row | undefined, b: Row | undefined): boolean {
  const comparable = (row: Row | undefined) => row && { ...row, bars: Object.entries(row.bars).filter(([, value]) => value.trim() !== "").sort(([a], [b]) => a.localeCompare(b)) };
  return JSON.stringify(comparable(a)) === JSON.stringify(comparable(b));
}

/**
 * Rows to show after the heat was loaded again. `base` is what `rows` were
 * typed over, `next` what was just loaded. A row the user changed is kept;
 * every other row follows `next`. A change is dropped when the position
 * now holds someone else, or when it is in `accepted` (what a save just
 * sent), so the saved row shows the stored text.
 */
export function mergeRows(base: Rows, next: Rows, rows: Rows, accepted?: Rows): Rows {
  const out: Rows = {};
  for (const [key, loaded] of Object.entries(next)) {
    const pos = Number(key);
    const typed = rows[pos];
    const keep = typed !== undefined && base[pos] !== undefined && typed.competitorId === loaded.competitorId &&
      !sameRow(typed, base[pos]) && !(accepted?.[pos] !== undefined && sameRow(typed, accepted[pos]));
    out[pos] = keep ? typed : loaded;
  }
  return out;
}
