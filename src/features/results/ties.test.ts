import { describe, expect, test } from "bun:test";
import type { RoundView } from "../../mygo";
import { changedPlaces, storedPlaces, tieRows } from "./ties";

const round = {
  ties: [
    { place: 2, competitorIds: ["b", "c"], ruled: false, extra: 1 },
    { place: 5, competitorIds: ["e", "f"], ruled: true, extra: 0 },
  ],
  standings: [
    { competitorId: "a", rank: 1 }, { competitorId: "b", rank: 2 }, { competitorId: "c", rank: 2 },
    { competitorId: "d", rank: 4 }, { competitorId: "e", rank: 5 }, { competitorId: "f", rank: 6 },
  ],
} as unknown as RoundView;

describe("tie rulings", () => {
  test("rows follow the ties", () => {
    expect(tieRows(round).map((r) => [r.competitorId, r.tie, r.first])).toEqual([["b", 0, true], ["c", 0, false], ["e", 1, true], ["f", 1, false]]);
  });
  test("stored places come from the standings", () => {
    expect(storedPlaces(round)).toEqual({ b: 2, c: 2, e: 5, f: 6 });
  });
  test("a change sends its whole tie only", () => {
    const stored = storedPlaces(round);
    expect(changedPlaces(round, stored, stored)).toEqual([]);
    expect(changedPlaces(round, stored, { ...stored, c: 3 })).toEqual([{ competitorId: "b", place: 2 }, { competitorId: "c", place: 3 }]);
  });
});
