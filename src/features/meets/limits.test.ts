import { expect, test } from "bun:test";
import { formOf, limitText, limitValue, sameForm } from "./limits";

test("limits read blank as none and keep two digits", () => {
  expect(limitValue("")).toBe(0);
  expect(limitValue("3")).toBe(3);
  expect(limitValue("2项")).toBe(2);
  expect(limitValue("123")).toBe(12);
  expect(limitValue("-1")).toBe(1);
  expect(limitText(0)).toBe("");
  expect(limitText(5)).toBe("5");
});

test("a meet's form compares by what it would save", () => {
  const blank = formOf();
  expect(blank.entryLimits).toEqual({ athleteEvents: 0, unitAthletes: 0, unitTeams: 0 });
  expect(sameForm(blank, { ...blank })).toBe(true);
  expect(sameForm(blank, { ...blank, name: "秋季运动会" })).toBe(false);
  expect(sameForm(blank, { ...blank, entryLimits: { ...blank.entryLimits, unitTeams: 2 } })).toBe(false);
});
