import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Groups, type GroupAssignment, type GroupInput } from "../../mygo";

export function useGroups(meetId: string) {
  return useData(() => Groups.list(meetId), [meetId], (c) => c.meetId === meetId, `groups:${meetId}`);
}

export function useGroupSuggestions(meetId: string) {
  return useData(() => Groups.suggestions(meetId), [meetId], (c) => c.meetId === meetId, `groupSuggestions:${meetId}`);
}

export const GroupsController = {
  save: (meetId: string, id: string, input: GroupInput) => act(() => Groups.save(meetId, id, input), "已保存"),
  remove: (meetId: string, id: string) => act(() => Groups.delete(meetId, id), "已删除"),
  reorder: (meetId: string, ids: string[]) => act(() => Groups.reorder(meetId, ids)),
  /** Moves events and combined events to a group ("" = ungrouped). */
  move: (meetId: string, groupId: string, eventIds: string[], combinedIds: string[]) =>
    act(() => Groups.change({ meetId, groupId, eventIds, combinedIds }), "已调整组别"),
  assign: (meetId: string, assignments: GroupAssignment[]) => act(() => Groups.assign({ meetId, assignments }), "已调整组别"),
};
