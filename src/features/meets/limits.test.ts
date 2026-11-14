import { expect, test } from "bun:test";
import { limitText, limitValue } from "./limits";

test("limits read blank as none and keep two digits", () => {
  expect(limitValue("")).toBe(0);
  expect(limitValue("3")).toBe(3);
  expect(limitValue("2项")).toBe(2);
  expect(limitValue("123")).toBe(12);
  expect(limitValue("-1")).toBe(1);
  expect(limitText(0)).toBe("");
  expect(limitText(5)).toBe("5");
});
