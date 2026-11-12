import type { EventKind, Status } from "../mygo";

export const kindLabels: Record<EventKind, string> = {
  track: "径赛",
  relay: "接力",
  distance: "远度",
  height: "高度",
 count: "计次",
 score: "计分",
};

export const statusLabels: Record<Status, string> = {
  OK: "有效",
  DNS: "弃权",
  DNF: "未完成",
  DQ: "犯规取消",
  NM: "无成绩",
};

export function dateRange(start: string, end: string): string {
  return end && end !== start ? `${start} – ${end}` : start;
}
