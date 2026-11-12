import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Entries, Events, type EntryInput, type EventInput } from "../../mygo";

export function useEvents(meetId: string) {
  return useData(() => Events.list(meetId), [meetId], (c) => c.meetId === meetId);
}

export function useEventDetail(eventId: string, meetId: string) {
  return useData(() => Events.get(eventId), [eventId], (c) => c.meetId === meetId && (c.eventId === eventId || c.eventId === ""));
}

export const EventsController = {
  save: (meetId: string, id: string, input: EventInput) => act(() => Events.save(meetId, id, input), "已保存"),
  count: (id: string) => act(() => Events.count(id)),
  remove: (id: string) => act(() => Events.delete(id), "已删除"),
  addEntries: (eventId: string, list: EntryInput[]) => act(() => Entries.add(eventId, list)),
  removeEntries: (eventId: string, athleteIds: string[]) => act(() => Entries.remove(eventId, athleteIds)),
  setSeed: (eventId: string, athleteId: string, seed: string) => act(() => Entries.setSeed(eventId, athleteId, seed)),
};
