import { Reports, TeamScoring, type CheckInRequest } from "../../mygo";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";

export const useReportTargets = (meetId: string) => useData(() => Reports.targets(meetId), [meetId], (c) => c.meetId === meetId, `reportTargets:${meetId}`);
/** Books and 报名表; not shown (enabled false) or nothing chosen gives an empty preview. */
export const useReportHTML = (kind: string, meetId: string, ids: string[], count: number, chosen: number, enabled: boolean, groupIds: string[] = []) =>
  useData(() => (!enabled || (count > 0 && chosen === 0) ? Promise.resolve("") : groupIds.length ? Reports.renderGrouped({ kind, meetId, eventIds: ids, groupIds }) : Reports.render(kind, meetId, ids)), [kind, meetId, ids.join(), count, chosen, enabled, groupIds.join()], (c) => c.meetId === meetId);
export const useCheckInTargets = (meetId: string) => useData(() => Reports.checkInTargets(meetId), [meetId], (c) => c.meetId === meetId, `checkInTargets:${meetId}`);
/** The per-heat sheets, chosen by round and day. */
export type SheetKind = "checkin" | "record";
const renderSheet = (kind: SheetKind, req: CheckInRequest) => (kind === "record" ? Reports.renderRecordSheet(req) : Reports.renderCheckIn(req));
/** A 检录单 or 成绩记录表; nothing chosen (chosen 0) shows an empty preview. */
export const useSheetHTML = (kind: SheetKind, req: CheckInRequest, chosen: number) =>
  useData(() => (chosen === 0 ? Promise.resolve("") : renderSheet(kind, req)), [kind, req.meetId, req.roundIds.join(), req.day, chosen], (c) => c.meetId === req.meetId);
export const ReportsController = {
  /** Prints the preview's laid-out sheets, so the pages are the ones shown. */
  printPages: (title: string, html: string) => act(() => Reports.printPages(title, html)),
};

export const useReportTeamScores = (meetId: string, enabled: boolean) => useData(() => enabled ? TeamScoring.get({ meetId }) : Promise.resolve(null), [meetId, enabled], (c) => enabled && c.meetId === meetId);
