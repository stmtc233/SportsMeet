import type { EntryLimits, Meet, MeetInput } from "../../mygo";

export const noLimits: EntryLimits = { athleteEvents: 0, unitAthletes: 0, unitTeams: 0 };

/** A limit as typed: blank means none. */
export function limitText(n: number): string {
  return n > 0 ? String(n) : "";
}

/** Keeps the digits of what was typed, at most two: blank is 0, no limit. */
export function limitValue(text: string): number {
  return Number(text.replace(/\D/g, "").slice(0, 2) || "0");
}

/** A meet as its form edits it; no meet is a blank one. */
export function formOf(meet?: Meet): Required<MeetInput> {
  if (!meet) return { name: "", startDate: "", endDate: "", location: "", note: "", entryLimits: noLimits };
  return { name: meet.name, startDate: meet.startDate, endDate: meet.endDate, location: meet.location, note: meet.note, entryLimits: meet.entryLimits ?? noLimits };
}

/** Whether two forms would save the same meet. */
export function sameForm(a: Required<MeetInput>, b: Required<MeetInput>): boolean {
  return a.name === b.name && a.startDate === b.startDate && a.endDate === b.endDate && a.location === b.location && a.note === b.note
    && a.entryLimits.athleteEvents === b.entryLimits.athleteEvents && a.entryLimits.unitAthletes === b.entryLimits.unitAthletes && a.entryLimits.unitTeams === b.entryLimits.unitTeams;
}
