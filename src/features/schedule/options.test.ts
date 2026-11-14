import { expect, test } from "bun:test";
import type { Timetable, TimetableHeat, TimetableRound } from "../../mygo";
import { scheduleOptions } from "./options";

function heat(id: string, effectiveAt: string, keepReason = "", durationMinutes: number | null = null): TimetableHeat {
  return { id, label: id, name: "", base: id, scheduledAt: "", durationMinutes, effectiveAt, scheduleInherited: false, scheduledText: "", durationText: "",
    endAt: "", endText: "", keepReason, people: 1, conflicts: [] };
}
function round(id: string, heats: TimetableHeat[]): TimetableRound {
  return { eventId: `e-${id}`, eventName: `项目${id}`, roundId: id, roundLabel: "决赛", day: "", scheduledAt: "", scheduledText: "", startAt: "", startText: "",
    heatCount: heats.length, people: heats.length, heats };
}

test("排时间选项按日程顺序，未安排的在后，取第一个未比赛的组", () => {
  const t: Timetable = { minRestMinutes: 0, conflicts: [],
    days: [{ day: "2026-11-12", rounds: [round("a", [heat("a1", "2026-11-12T09:00", "已有成绩", 5), heat("a2", "2026-11-12T09:05", "", 6)])] },
      { day: "2026-11-13", rounds: [round("b", [])] }],
    unscheduled: [round("c", [heat("c1", "")])] };
  expect(scheduleOptions(t)).toEqual([
    { eventId: "e-a", roundId: "a", label: "项目a 决赛", firstAt: "2026-11-12T09:05", minutes: 6 },
    { eventId: "e-b", roundId: "b", label: "项目b 决赛", firstAt: "", minutes: null },
    { eventId: "e-c", roundId: "c", label: "项目c 决赛", firstAt: "", minutes: null },
  ]);
});
