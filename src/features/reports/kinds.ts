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

/** The lasting view-state key of the report on 打印; other pages set it to open one there. */
export const reportKindKey = "reports:kind";
