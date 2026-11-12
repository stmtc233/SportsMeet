import { StMuted, StTag } from "../../design";

/**
 * A heat's effective time in its title: its own time as a tag; the round's
 * start, which it follows, as plain secondary text so it does not read as
 * set for the heat.
 */
export function HeatTime({ text, inherited }: { text: string; inherited: boolean }) {
  if (!text) return null;
  if (!inherited) return <StTag>{text}</StTag>;
  return <span title="沿用轮次开始时间" data-inherited><StMuted>{text}</StMuted></span>;
}
