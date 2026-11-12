import { splitScheduled, type WallTimeFields } from "../../lib/wallTime";
import type { HeatView } from "../../mygo";

/**
 * The fields the actual-start dialog opens with: the recorded start, else
 * the planned date with no time, else the event's day.
 */
export function startedFields(heat: Pick<HeatView, "startedAt" | "effectiveAt">, day: string): WallTimeFields {
  if (heat.startedAt) return splitScheduled(heat.startedAt);
  return { date: splitScheduled(heat.effectiveAt).date || day, time: "" };
}
