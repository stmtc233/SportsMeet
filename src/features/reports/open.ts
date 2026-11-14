import { useNav } from "../../app/nav";
import { remember } from "../../lib/viewState";
import { reportKindKey, reportScopeKey, type ReportKind } from "./kinds";

/** Opens 打印 at a report, optionally for some rounds or events only. */
export function useOpenReport(meetId: string) {
  const { go } = useNav();
  return (kind: ReportKind, ids?: string[]) => {
    remember(reportKindKey, kind, true);
    remember(reportScopeKey(meetId), ids ? { kind, ids } : undefined);
    go({ page: "meet", meetId, tab: "reports" });
  };
}
