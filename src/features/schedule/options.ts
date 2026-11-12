import type { Timetable } from "../../mygo";
import type { ScheduleOption } from "../arrange/ScheduleDialog";

/** Every round as the multi-round dialog lists it: the timetable's order, unscheduled ones last. */
export function scheduleOptions(t: Timetable): ScheduleOption[] {
  return [...t.days.flatMap((d) => d.rounds), ...t.unscheduled].map((r) => {
    const open = r.heats.find((h) => !h.keepReason);
    return { eventId: r.eventId, roundId: r.roundId, label: `${r.eventName} ${r.roundLabel}`, firstAt: open?.effectiveAt ?? "", minutes: open?.durationMinutes ?? null };
  });
}
