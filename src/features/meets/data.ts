import { act } from "../../lib/action";
import { useOptimistic } from "../../lib/optimistic";
import { useData } from "../../lib/useData";
import { Meets, type MeetSettings } from "../../mygo";

export function useMeets() {
  return useData(() => Meets.list(), [], (c) => c.kind === "meets", "meets");
}

export function useMeet(meetId: string) {
  return useData(() => Meets.get(meetId), [meetId], (c) => c.kind === "meets" && c.meetId === meetId, `meet:${meetId}`);
}

/** The meet's arrange and print settings. */
export function useMeetSettings(meetId: string) {
  return useData(() => Meets.settings(meetId), [meetId], (c) => c.kind === "meets" && c.meetId === meetId, `meetSettings:${meetId}`);
}

/** The meet's settings, each control taking effect at once (lib/optimistic.ts). */
export function useOptimisticMeetSettings(meetId: string) {
  const { data, error } = useMeetSettings(meetId);
  return { ...useOptimistic(data ?? undefined, (next: MeetSettings) => act(() => Meets.saveSettings(meetId, next))), error };
}
