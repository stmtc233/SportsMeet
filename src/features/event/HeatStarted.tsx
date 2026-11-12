import { StMuted, StTag } from "../../design";

/**
 * A heat's actual start and how it compares with the plan; texts come from
 * Go. Where space runs out it is cut short, the whole text in its tooltip.
 */
export function HeatStarted({ text, delta }: { text: string; delta: string }) {
  if (!text) return null;
  return (
    <span className="heat-started" data-started title={`开赛 ${text}${delta ? ` ${delta}` : ""}`}>
      <StMuted>开赛 {text}</StMuted>
      {delta && <StTag>{delta}</StTag>}
    </span>
  );
}
