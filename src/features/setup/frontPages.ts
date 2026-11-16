import type { StOption } from "../../design";

/** The reports a front page can go in front of; the ids are Go's (printstyle.ReportKinds), in the reports' order. */
export const frontPageReports: StOption<string>[] = [
  { value: "entries", label: "报名表" },
  { value: "startlist", label: "秩序册" },
  { value: "results", label: "成绩册" },
  { value: "team_scores", label: "团体总分" },
  { value: "checkin", label: "检录单" },
  { value: "recordsheet", label: "成绩记录表" },
];

/** Where a front page is printed, for its row: no report named means every report. */
export function reportsText(reports: string[]): string {
  const names = reports.map((id) => frontPageReports.find((r) => r.value === id)?.label).filter((n) => n !== undefined);
  return names.length === 0 ? "印在全部报表前" : `印在${names.join("、")}前`;
}

/** The order after moving the page at index one place up (-1) or down (1); unchanged at the ends. */
export function moved(ids: string[], index: number, by: -1 | 1): string[] {
  const to = index + by;
  if (index < 0 || index >= ids.length || to < 0 || to >= ids.length) return ids;
  const next = [...ids];
  [next[index], next[to]] = [next[to]!, next[index]!];
  return next;
}
