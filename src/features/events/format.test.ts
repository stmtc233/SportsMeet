import { describe, expect, test } from "bun:test";
import { formatKey, formatOptions, withFormat, withRound } from "./format";

describe("赛制", () => {
  test("lists the usual formats", () => {
    expect(formatOptions([]).map((o) => o.label)).toEqual(["直接决赛", "预赛 + 决赛", "预赛 + 半决赛 + 决赛", "预赛 + 次赛 + 半决赛 + 决赛"]);
  });
  test("adds the event's own format when it is another", () => {
    const rounds = [{ stage: "semifinal", advanceByPlace: 3, advanceByMark: 2 }];
    expect(formatKey(rounds)).toBe("semifinal");
    expect(formatOptions(rounds).at(-1)).toEqual({ value: "semifinal", label: "半决赛 + 决赛" });
  });
  test("keeps the numbers of rounds that stay", () => {
    const rounds = withRound(withFormat([], "heat"), "heat", { advanceByPlace: 3 });
    expect(withFormat(rounds, "heat+semifinal")).toEqual([
      { stage: "heat", advanceByPlace: 3, advanceByMark: 2 },
      { stage: "semifinal", advanceByPlace: 2, advanceByMark: 2 },
    ]);
    expect(withFormat(rounds, "")).toEqual([]);
  });
});
