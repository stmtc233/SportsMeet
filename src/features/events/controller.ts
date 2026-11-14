import { useEffect, useState } from "react";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Entries, Events, Marks, type Event, type EventRules, type Gender, type EntryInput, type TeamInput, type EventInput } from "../../mygo";

export function useEvents(meetId: string) {
  return useData(() => Events.list(meetId), [meetId], (c) => c.meetId === meetId, `events:${meetId}`);
}

export function useEventDetail(eventId: string, meetId: string) {
  return useData(() => Events.get(eventId), [eventId], (c) => c.meetId === meetId && (c.eventId === eventId || c.eventId === ""), `event:${eventId}`);
}

export function useEventPresets() {
  return useData(() => Events.presets(), [], () => false, "presets");
}

export function useEventRules(input: EventInput, enabled: boolean) {
  const [rules, setRules] = useState<EventRules>();
  useEffect(() => {
    if (!enabled) return;
    let live = true;
    void Events.rules(input).then((r) => { if (live) setRules(r ?? undefined); }).catch(() => {});
    return () => { live = false; };
  }, [input, enabled]);
  return rules;
}

export function useTeamSizeWarning(event: Event, count: number) {
  const [warning, setWarning] = useState("");
  useEffect(() => {
    let live = true;
    void Events.teamSizeWarning(event, count).then((w) => { if (live) setWarning(w); }).catch(() => {});
    return () => { live = false; };
  }, [event, count]);
  return warning;
}

/** The 报名限制 entering ids would exceed, from Go; nothing while ids is empty. */
export function useEntryLimitWarnings(eventId: string, ids: string[]) {
  const [warnings, setWarnings] = useState<string[]>([]);
  const key = ids.join("\n"); // the request follows the ids, not the array passed in
  useEffect(() => {
    let live = true;
    if (key === "") setWarnings((old) => (old.length ? [] : old));
    else void Entries.limitWarnings(eventId, key.split("\n")).then((list) => { if (live) setWarnings(list ?? []); }).catch(() => {});
    return () => { live = false; };
  }, [eventId, key]);
  return warnings;
}

export const EventsController = {
  eventName: (gender: string, discipline: string, group: string) => Marks.eventName(gender as Gender, discipline, group),
  save: (meetId: string, id: string, input: EventInput) => act(() => Events.save(meetId, id, input), "已保存"),
  saveTeam: (eventId: string, id: string, input: TeamInput) => act(() => Events.saveTeam(eventId, id, input), "已保存"),
  deleteTeam: (eventId: string, id: string) => act(() => Events.deleteTeam(eventId, id), "已删除"),
  count: (id: string) => act(() => Events.count(id)),
  remove: (id: string) => act(() => Events.delete(id), "已删除"),
  addEntries: (eventId: string, list: EntryInput[]) => act(() => Entries.add(eventId, list)),
  removeEntries: (eventId: string, competitorIds: string[]) => act(() => Entries.remove(eventId, competitorIds)),
  setSeed: (eventId: string, competitorId: string, seed: string) => act(() => Entries.setSeed(eventId, competitorId, seed)),
};

export function useTeamWarnings(event: Event, memberIds: string[]) {
  const [warnings, setWarnings] = useState<string[]>([]);
  useEffect(() => {
    let live = true;
    void Events.teamWarnings(event.id, memberIds).then((list) => { if (live) setWarnings(list); }).catch(() => {});
    return () => { live = false; };
  }, [event, memberIds]);
  return warnings;
}
