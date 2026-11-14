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
  /** Saves the group, then sets which units belong to it (assignments get the group's id, new or not). */
  save: (meetId: string, id: string, input: GroupInput, assignments?: (groupId: string) => GroupAssignment[]) =>
    act(async () => {
      const group = await Groups.save(meetId, id, input);
      const list = group && assignments ? assignments(group.id) : [];
      if (list.length > 0) await Groups.assign({ meetId, assignments: list });
      return group;
    }, "已保存"),
  /** A new group by name; the group, or undefined when it failed. */
  create: async (meetId: string, name: string) => {
    const group = await act(() => Groups.save(meetId, "", { name }), "已添加组别");
    return group && typeof group === "object" ? group : undefined;
  },
  remove: (meetId: string, id: string) => act(() => Groups.delete(meetId, id), "已删除"),
  reorder: (meetId: string, ids: string[]) => act(() => Groups.reorder(meetId, ids)),
  /** Moves events and combined events to a group ("" = ungrouped). */
  move: (meetId: string, groupId: string, eventIds: string[], combinedIds: string[]) =>
    act(() => Groups.change({ meetId, groupId, eventIds, combinedIds }), "已调整组别"),
  assign: (meetId: string, assignments: GroupAssignment[]) => act(() => Groups.assign({ meetId, assignments }), "已调整组别"),
};
