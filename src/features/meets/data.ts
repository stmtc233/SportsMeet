import { useData } from "../../lib/useData";
import { Meets } from "../../mygo";

export function useMeets() {
  return useData(() => Meets.list(), [], (c) => c.kind === "meets");
}

export function useMeet(meetId: string) {
  return useData(() => Meets.get(meetId), [meetId], (c) => c.kind === "meets" && c.meetId === meetId);
}
