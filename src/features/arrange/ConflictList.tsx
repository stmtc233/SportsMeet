import { StButton, StTag } from "../../design";
import type { ConflictView, ScheduleHeatRef } from "../../mygo";

/**
 * Athlete time conflicts, one per row: who, what, and both heats in full.
 * On a narrow window a row stacks instead of cutting text off. onOpen adds
 * a 「编排」 button to each heat.
 */
export function ConflictList({ conflicts, onOpen }: { conflicts: ConflictView[]; onOpen?: (heat: ScheduleHeatRef) => void }) {
  return (
    <ul className="conflicts" data-conflicts>
      {conflicts.map((c) => (
        <li key={`${c.athleteId}|${c.first.heatId}|${c.second.heatId}`} className="conflicts__row">
          <span className="conflicts__who">
            <span className="cell-strong">{c.athleteName}</span>
            <StTag tone={c.overlap ? "danger" : "warning"}>{c.text}</StTag>
          </span>
          {[c.first, c.second].map((heat, i) => (
            <span key={i} className="conflicts__heat">
              <span className="conflicts__text">{heat.text}</span>
              {onOpen && <StButton size="small" variant="ghost" onClick={() => onOpen(heat)}>编排</StButton>}
            </span>
          ))}
        </li>
      ))}
    </ul>
  );
}

