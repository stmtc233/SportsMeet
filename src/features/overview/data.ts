import { useData } from "../../lib/useData";
import { Meets } from "../../mygo";

/** Where the meet stands; any change in the meet, or to the settings (the minimum rest), reads it again. */
export function useOverview(meetId: string) {
  return useData(() => Meets.overview(meetId), [meetId], (c) => c.meetId === meetId || c.kind === "settings", `overview:${meetId}`);
}
