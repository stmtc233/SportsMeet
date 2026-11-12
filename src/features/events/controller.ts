import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Entries, Events, type EntryInput, type TeamInput, type EventInput } from "../../mygo";

export function useEvents(meetId: string) {
  return useData(() => Events.list(meetId), [meetId], (c) => c.meetId === meetId);
}

export function useEventDetail(eventId: string, meetId: string) {
  return useData(() => Events.get(eventId), [eventId], (c) => c.meetId === meetId && (c.eventId === eventId || c.eventId === ""));
}

export const EventsController = {
  save: (meetId: string, id: string, input: EventInput) => act(() => Events.save(meetId, id, input), "已保存"),
  saveTeam: (eventId: string, id: string, input: TeamInput) => act(() => Events.saveTeam(eventId, id, input), "已保存"),
  deleteTeam: (eventId: string, id: string) => act(() => Events.deleteTeam(eventId, id), "已删除"),
  count: (id: string) => act(() => Events.count(id)),
  remove: (id: string) => act(() => Events.delete(id), "已删除"),
  addEntries: (eventId: string, list: EntryInput[]) => act(() => Entries.add(eventId, list)),
  removeEntries: (eventId: string, competitorIds: string[]) => act(() => Entries.remove(eventId, competitorIds)),
  setSeed: (eventId: string, competitorId: string, seed: string) => act(() => Entries.setSeed(eventId, competitorId, seed)),
};
