/** The reports 打印 offers, in the order of its picker. */
export type ReportKind = "startlist" | "results" | "entries" | "checkin" | "record" | "team_scores";

export const reportKinds: { value: ReportKind; label: string }[] = [
  { value: "startlist", label: "秩序册" },
  { value: "results", label: "成绩册" },
  { value: "entries", label: "报名表" },
  { value: "checkin", label: "检录单" },
  { value: "record", label: "成绩记录表" },
  { value: "team_scores", label: "团体总分" },
];

/** 打印's list of reports, by the part of the meet they are for (as on 总览). */
export const reportGroups: { title: string; kinds: ReportKind[] }[] = [
  { title: "赛前", kinds: ["entries", "startlist"] },
  { title: "比赛", kinds: ["checkin", "record"] },
  { title: "赛后", kinds: ["results", "team_scores"] },
];

/** The lasting view-state key of the report on 打印; other pages set it to open one there. */
export const reportKindKey = "reports:kind";

/**
 * What another page asks 打印 to print: the rounds of 检录单 and 成绩记录表,
 * the events of the other reports. 打印 ticks just those once, then forgets it.
 */
export type ReportScope = { kind: ReportKind; ids: string[] };

export const reportScopeKey = (meetId: string) => `reports:scope:${meetId}`;
