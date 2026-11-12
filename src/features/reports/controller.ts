import { Reports, type CheckInRequest } from "../../mygo";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";

export const useReportTargets = (meetId: string) => useData(() => Reports.targets(meetId), [meetId], (c) => c.meetId === meetId);
/** A 秩序册 or 成绩册; not shown (enabled false) or nothing chosen gives an empty preview. */
export const useReportHTML = (kind: string, meetId: string, ids: string[], count: number, chosen: number, enabled: boolean) =>
  useData(() => (!enabled || (count > 0 && chosen === 0) ? Promise.resolve("") : Reports.render(kind, meetId, ids)), [kind, meetId, ids.join(), count, enabled], (c) => c.meetId === meetId || c.kind === "settings");
export const useCheckInTargets = (meetId: string) => useData(() => Reports.checkInTargets(meetId), [meetId], (c) => c.meetId === meetId);
/** The 检录单; nothing chosen (chosen 0) shows an empty preview. */
export const useCheckInHTML = (req: CheckInRequest, chosen: number) =>
  useData(() => (chosen === 0 ? Promise.resolve("") : Reports.renderCheckIn(req)), [req.meetId, req.roundIds.join(), req.day, chosen], (c) => c.meetId === req.meetId || c.kind === "settings");
export const ReportsController = {
  print: (kind: string, meetId: string, ids: string[]) => act(() => Reports.print(kind, meetId, ids)),
  printCheckIn: (req: CheckInRequest) => act(() => Reports.printCheckIn(req)),
};
