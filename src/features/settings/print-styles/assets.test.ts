import { describe, expect, test } from "bun:test";
import { sizeText } from "./assets";

describe("sizeText", () => {
  test("picks the unit", () => {
    expect(sizeText(800)).toBe("800 B");
    expect(sizeText(48 * 1024)).toBe("48 KB");
    expect(sizeText(1.5 * 1024 * 1024)).toBe("1.5 MB");
  });
});
