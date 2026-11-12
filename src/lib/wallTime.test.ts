import { describe, expect, test } from "bun:test";
import { joinScheduled, splitScheduled, today } from "./wallTime";

describe("scheduled time fields", () => {
  test("split and join round-trip", () => {
    expect(splitScheduled("2026-11-12T09:30")).toEqual({ date: "2026-11-12", time: "09:30" });
    expect(joinScheduled("2026-11-12", "09:30")).toBe("2026-11-12T09:30");
  });

  test("unscheduled gives empty fields", () => {
    expect(splitScheduled("")).toEqual({ date: "", time: "" });
  });

  test("a missing date or time is unscheduled", () => {
    expect(joinScheduled("2026-11-12", "")).toBe("");
    expect(joinScheduled("", "09:30")).toBe("");
  });

  test("today is the local calendar day", () => {
    expect(today(new Date(2026, 0, 5, 23, 59))).toBe("2026-01-05");
  });
});
