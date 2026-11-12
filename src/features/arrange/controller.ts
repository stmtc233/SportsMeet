import { useEffect, useMemo, useState } from "react";
import { Arrange, type BoardSave, type HeatConflict, type PlanRequest, type RoundStart } from "../../mygo";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";

export const ArrangeController = {
  save: (input: BoardSave) => Arrange.save(input),
  plan: (input: PlanRequest) => act(() => Arrange.plan(input)),
  /** Checks the heat dialog's time and duration; the texts come back with it. */
  heatPlan: (roundStart: string, at: string, minutes: string, day: string) => act(() => Arrange.heatPlan(roundStart, at, minutes, day)),
  setRoundStart: (input: RoundStart) => act(() => Arrange.setRoundStart(input), "已保存"),
};

/** What a time dialog warns about before saving (a day outside the meet); "" = nothing. */
export function useScheduleWarning(eventId: string, at: string) {
  const [warning, setWarning] = useState("");
  useEffect(() => {
    if (!at) { setWarning(""); return; }
    let live = true;
    void Arrange.scheduleWarning(eventId, at).then((w) => { if (live) setWarning(w); }).catch(() => { if (live) setWarning(""); });
    return () => { live = false; };
  }, [eventId, at]);
  return warning;
}

/**
 * The athlete time conflicts of the event's heats, by heat id. Any change
 * in the meet can move them, and so can the minimum rest in the settings.
 */
export function useHeatConflicts(eventId: string, meetId: string): Map<string, HeatConflict[]> {
  const { data } = useData(() => Arrange.heatConflicts(eventId), [eventId], (c) => c.meetId === meetId || c.kind === "settings");
  return useMemo(() => new Map((data ?? []).map((h) => [h.heatId, h.conflicts])), [data]);
}
