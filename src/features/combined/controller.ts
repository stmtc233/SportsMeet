import { Combined, type CombinedInput, type CombinedRulesRequest, type Event, type EventInput, type PointRuleInput } from "../../mygo";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { blankEvent } from "../events/form";

export const useCombinedList = (meetId: string) => useData(() => Combined.list(meetId), [meetId], (c) => c.meetId === meetId);
export const useCombinedDetail = (id: string, meetId: string) => useData(() => Combined.get(id), [id], (c) => c.meetId === meetId);
export const CombinedController = {
  get: (id: string) => act(() => Combined.get(id)),
  save: (meetId: string, id: string, input: CombinedInput) => act(() => Combined.save(meetId, id, input), "已保存"),
  entries: (id: string, athleteIds: string[]) => act(() => Combined.entries(id, athleteIds), "已保存"),
  remove: (id: string) => act(() => Combined.delete(id), "已删除"),
  parseRule: (event: EventInput, rule: PointRuleInput) => act(() => Combined.parseRule(event, rule)),
  previewRules: (request: CombinedRulesRequest) => act(() => Combined.previewRules(request)),
  applyRules: (request: CombinedRulesRequest, hash: string) => act(() => Combined.applyRules(request, hash, true), "已保存"),
};
export function inputFromEvent(e: Event): Required<EventInput> {
  return { ...blankEvent, name: e.name, gender: e.gender, discipline: e.discipline, groupLabel: e.groupLabel, kind: e.kind, date: e.date, laneCount: e.laneCount, attemptCount: e.attemptCount, note: e.note, startMode: e.startMode, distanceMode: e.distanceMode, relayLegCount: e.relayLegCount, participantMode: e.participantMode ?? "athlete", teamSize: e.teamSize ?? 0, scoreDecimals: e.scoreDecimals ?? 0, rankDirection: e.rankDirection ?? "", attemptMode: e.attemptMode ?? "" };
}
export const emptyRule = (): PointRuleInput => ({ table: "", invalidPolicies: { DNS: "zero", DNF: "zero", DQ: "zero", NM: "zero" } });
