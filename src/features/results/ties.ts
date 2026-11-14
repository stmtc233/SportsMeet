// Tie rulings as chosen in the dialog (docs/agents/frontend.md「成绩页」).
// Pure, so it is tested without a page.
import type { RoundView, TiePlace } from "../../mygo";

/** One tied competitor, in the round's tie order. */
export interface TieRow {
  competitorId: string;
  tie: number; // index into round.ties
  first: boolean; // first row of its tie
}

export function tieRows(round: RoundView): TieRow[] {
  return round.ties.flatMap((t, tie) => t.competitorIds.map((competitorId, i) => ({ competitorId, tie, first: i === 0 })));
}

/** Each tied competitor's place as stored now. */
export function storedPlaces(round: RoundView): Record<string, number> {
  const rank = new Map(round.standings.map((s) => [s.competitorId, s.rank]));
  return Object.fromEntries(round.ties.flatMap((t) => t.competitorIds.map((id) => [id, rank.get(id) ?? t.place])));
}

/** Every competitor of each tie with a changed place: Go rules whole ties. */
export function changedPlaces(round: RoundView, stored: Record<string, number>, chosen: Record<string, number>): TiePlace[] {
  return round.ties
    .filter((t) => t.competitorIds.some((id) => chosen[id] !== stored[id]))
    .flatMap((t) => t.competitorIds.map((competitorId) => ({ competitorId, place: chosen[competitorId] ?? t.place })));
}
