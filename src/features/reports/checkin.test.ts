import { describe, expect, test } from "bun:test";
import type { CheckInTargets } from "../../mygo";
import { checkInRows, checkInSelection } from "./checkin";

const targets: CheckInTargets = {
  days: ["2026-11-12", "2026-11-13"],
  events: [
    { id: "e1", name: "男子100m", rounds: [
      { id: "r1", label: "预赛", heatCount: 3, days: ["2026-11-12"] },
      { id: "r2", label: "决赛", heatCount: 1, days: ["2026-11-13"] },
    ] },
    { id: "e2", name: "女子跳远", rounds: [{ id: "r3", label: "决赛", heatCount: 1, days: [] }] },
  ],
};

describe("checkInRows", () => {
  test("every day: all rounds, the round named when an event has several", () => {
    expect(checkInRows(targets, "")).toEqual([
      { id: "r1", label: "男子100m 预赛" },
      { id: "r2", label: "男子100m 决赛" },
      { id: "r3", label: "女子跳远" },
    ]);
  });
  test("a day: only rounds with heats on it", () => {
    expect(checkInRows(targets, "2026-11-13").map((r) => r.id)).toEqual(["r2"]);
  });
  test("nothing loaded", () => {
    expect(checkInRows(undefined, "")).toEqual([]);
  });
});

describe("checkInSelection", () => {
  const rows = checkInRows(targets, "");
  test("all ticked asks for every round", () => {
    expect(checkInSelection(rows, new Set())).toEqual({ chosen: ["r1", "r2", "r3"], roundIds: [] });
  });
  test("some unticked asks for the rest", () => {
    expect(checkInSelection(rows, new Set(["r2"]))).toEqual({ chosen: ["r1", "r3"], roundIds: ["r1", "r3"] });
  });
  test("a round skipped on another day does not count", () => {
    expect(checkInSelection(checkInRows(targets, "2026-11-13"), new Set(["r1"]))).toEqual({ chosen: ["r2"], roundIds: [] });
  });
});
