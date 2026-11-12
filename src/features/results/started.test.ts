import { describe, expect, test } from "bun:test";
import { startedFields } from "./started";

describe("startedFields", () => {
  test("the recorded start", () => {
    expect(startedFields({ startedAt: "2026-11-13T09:42", effectiveAt: "2026-11-12T09:30" }, "2026-11-12")).toEqual({ date: "2026-11-13", time: "09:42" });
  });
  test("the planned date without a time", () => {
    expect(startedFields({ startedAt: "", effectiveAt: "2026-11-13T09:30" }, "2026-11-12")).toEqual({ date: "2026-11-13", time: "" });
  });
  test("the event's day", () => {
    expect(startedFields({ startedAt: "", effectiveAt: "" }, "2026-11-12")).toEqual({ date: "2026-11-12", time: "" });
  });
});
