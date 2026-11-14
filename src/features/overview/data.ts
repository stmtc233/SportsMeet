import { useData } from "../../lib/useData";
import { Meets } from "../../mygo";

/** Where the meet stands; any change in the meet (its settings included) reads it again. */
export function useOverview(meetId: string) {
  return useData(() => Meets.overview(meetId), [meetId], (c) => c.meetId === meetId, `overview:${meetId}`);
}
