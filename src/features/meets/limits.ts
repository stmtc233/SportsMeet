import type { EntryLimits } from "../../mygo";

export const noLimits: EntryLimits = { athleteEvents: 0, unitAthletes: 0, unitTeams: 0 };

/** A limit as typed: blank means none. */
export function limitText(n: number): string {
  return n > 0 ? String(n) : "";
}

/** Keeps the digits of what was typed, at most two: blank is 0, no limit. */
export function limitValue(text: string): number {
  return Number(text.replace(/\D/g, "").slice(0, 2) || "0");
}
