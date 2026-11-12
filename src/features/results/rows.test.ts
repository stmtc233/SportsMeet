import { describe, expect, test } from "bun:test";
import { mergeRows, toRow, type Row, type Rows } from "./rows";

const row = (competitorId: string, mark = "", patch: Partial<Row> = {}): Row => ({ competitorId, status: "", mark, attempts: [], bars: {}, ...patch });

describe("mergeRows", () => {
  test("typed rows are kept, untouched rows follow the reload", () => {
    const base: Rows = { 1: row("a"), 2: row("b") };
    const typed: Rows = { 1: row("a", "12.3"), 2: row("b") };
    const next: Rows = { 1: row("a"), 2: row("b", "13.00") };
    expect(mergeRows(base, next, typed)).toEqual({ 1: row("a", "12.3"), 2: row("b", "13.00") });
  });

  test("a saved row shows the stored text", () => {
    const base: Rows = { 1: row("a"), 2: row("b") };
    const typed: Rows = { 1: row("a", "12.3"), 2: row("b", "13") };
    const next: Rows = { 1: row("a", "12.30"), 2: row("b") };
    // Only position 1 was sent.
    expect(mergeRows(base, next, typed, { 1: typed[1]! })).toEqual({ 1: row("a", "12.30"), 2: row("b", "13") });
  });

  test("typing after sending is kept", () => {
    const base: Rows = { 1: row("a") };
    const sent: Rows = { 1: row("a", "12.3") };
    const typed: Rows = { 1: row("a", "12.31") };
    expect(mergeRows(base, { 1: row("a", "12.30") }, typed, sent)).toEqual(typed);
  });

  test("input for someone no longer at the position is dropped", () => {
    const base: Rows = { 1: row("a") };
    const typed: Rows = { 1: row("a", "12.3") };
    expect(mergeRows(base, { 1: row("c") }, typed)).toEqual({ 1: row("c") });
  });

  test("positions come from the reload", () => {
    const base: Rows = { 1: row("a"), 2: row("b") };
    const typed: Rows = { 1: row("a"), 2: row("b", "13") };
    expect(mergeRows(base, { 1: row("a"), 3: row("d") }, typed)).toEqual({ 1: row("a"), 3: row("d") });
  });

  test("status and attempts count as typed", () => {
    const base: Rows = { 1: row("a", "", { attempts: ["", ""] }), 2: row("b") };
    const typed: Rows = { 1: row("a", "", { attempts: ["5.20", ""] }), 2: row("b", "", { status: "DNS" }) };
    const next: Rows = { 1: row("a", "", { attempts: ["", ""] }), 2: row("b") };
    expect(mergeRows(base, next, typed)).toEqual(typed);
  });
});

test("toRow shows a typed status, not one that follows from attempts", () => {
  const result = { mark: null, markText: "", rank: null, qualify: "", note: "", bars: [] };
  expect(toRow({ position: 1, athleteId: "a", competitorId: "a", result: { ...result, status: "DNS", attempts: [] } }, 3))
    .toEqual({ competitorId: "a", status: "DNS", mark: "", attempts: ["", "", ""], bars: {} });
  expect(toRow({ position: 1, athleteId: "a", competitorId: "a", result: { ...result, status: "NM", attempts: ["X", "X"] } }, 3))
    .toEqual({ competitorId: "a", status: "", mark: "", attempts: ["X", "X", ""], bars: {} });
});
