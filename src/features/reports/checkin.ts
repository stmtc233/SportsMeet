import type { CheckInTargets } from "../../mygo";

/** One round the 检录单 can print, as the side list shows it. */
export interface CheckInRow {
  id: string;
  label: string;
}

/**
 * The rounds listed for a day ("" = every day): those with heats planned
 * on it, labelled by event, plus the round when the event has more than one.
 */
export function checkInRows(targets: CheckInTargets | null | undefined, day: string): CheckInRow[] {
  const rows: CheckInRow[] = [];
  for (const e of targets?.events ?? []) {
    for (const r of e.rounds) {
      if (day && !r.days.includes(day)) continue;
      rows.push({ id: r.id, label: e.rounds.length > 1 ? `${e.name} ${r.label}` : e.name });
    }
  }
  return rows;
}

/**
 * The listed rounds still ticked, and the rounds to ask Go for: none when
 * every listed round is ticked, which prints them all (with the day filter
 * doing the rest).
 */
export function checkInSelection(rows: CheckInRow[], skipped: ReadonlySet<string>): { chosen: string[]; roundIds: string[] } {
  const chosen = rows.filter((r) => !skipped.has(r.id)).map((r) => r.id);
  return { chosen, roundIds: chosen.length === rows.length ? [] : chosen };
}
