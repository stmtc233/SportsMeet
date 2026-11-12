import { Reports } from "../../mygo";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";

export const useReportTargets = (meetId: string) => useData(() => Reports.targets(meetId), [meetId], (c) => c.meetId === meetId);
export const useReportHTML = (kind: string, meetId: string, ids: string[], count: number, chosen: number) => useData(() => count > 0 && chosen === 0 ? Promise.resolve("") : Reports.render(kind, meetId, ids), [kind, meetId, ids.join(), count], (c) => c.meetId === meetId || c.kind === "settings");
export const ReportsController = { print: (kind: string, meetId: string, ids: string[]) => act(() => Reports.print(kind, meetId, ids)) };
