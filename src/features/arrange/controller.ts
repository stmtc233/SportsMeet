import { useEffect, useMemo, useState } from "react";
import { Arrange, events, type BoardSave, type HeatConflict, type HeatConflicts, type PlanRequest, type RoundStart } from "../../mygo";
import { act } from "../../lib/action";
import { draftTimes, type DraftRound } from "./draft";

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
 * The athlete time conflicts of the draft's heats, by heat key: Go checks
 * the draft (unsaved changes included) against the meet's saved times.
 * Draft changes and data changes in the meet (or the minimum rest) are
 * merged into one call after a short pause.
 */
export function useDraftConflicts(eventId: string, meetId: string, rounds: DraftRound[]): Map<string, HeatConflict[]> {
  const request = JSON.stringify(draftTimes(eventId, rounds));
  const [tick, setTick] = useState(0);
  const [data, setData] = useState<HeatConflicts[]>([]);
  useEffect(() => events.dataChanged.on((c) => { if (c.meetId === meetId || c.kind === "settings") setTick((n) => n + 1); }), [meetId]);
  useEffect(() => {
    let live = true;
    const timer = setTimeout(() => {
      void Arrange.draftConflicts(JSON.parse(request)).then((list) => { if (live) setData(list ?? []); }).catch(() => { if (live) setData([]); });
    }, 300);
    return () => { live = false; clearTimeout(timer); };
  }, [request, tick]);
  return useMemo(() => new Map(data.map((h) => [h.heatId, h.conflicts])), [data]);
}
