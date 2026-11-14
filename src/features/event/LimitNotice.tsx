import { StNotice } from "../../design";

const shownAtMost = 3;

/** The 报名限制 a pick would exceed: pointed out, never blocking. */
export function LimitNotice({ warnings }: { warnings: string[] }) {
  if (warnings.length === 0) return null;
  const rest = warnings.length - shownAtMost;
  return (
    <StNotice tone="warning">
      {warnings.slice(0, shownAtMost).map((w) => <div key={w}>{w}</div>)}
      {rest > 0 && <div>另有 {rest} 条</div>}
    </StNotice>
  );
}
