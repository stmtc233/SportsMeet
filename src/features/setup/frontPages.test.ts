import { describe, expect, test } from "bun:test";
import { moved, reportsText } from "./frontPages";

describe("reportsText", () => {
  test("no report named is every report", () => {
    expect(reportsText([])).toBe("印在全部报表前");
  });
  test("names the reports", () => {
    expect(reportsText(["startlist", "results"])).toBe("印在秩序册、成绩册前");
  });
});

describe("moved", () => {
  test("swaps with the neighbour", () => {
    expect(moved(["a", "b", "c"], 1, -1)).toEqual(["b", "a", "c"]);
    expect(moved(["a", "b", "c"], 1, 1)).toEqual(["a", "c", "b"]);
  });
  test("stays at the ends", () => {
    expect(moved(["a", "b"], 0, -1)).toEqual(["a", "b"]);
    expect(moved(["a", "b"], 1, 1)).toEqual(["a", "b"]);
  });
});
