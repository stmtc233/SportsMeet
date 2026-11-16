import { StNotice } from "../../design";

const shown = 3;

/** What the meet's print style could not do (a missing image, a template that failed); the first few, then how many more. */
export function ReportWarnings({ warnings }: { warnings: string[] }) {
  if (warnings.length === 0) return null;
  const rest = warnings.length - shown;
  return (
    <div className="report-warnings">
      {warnings.slice(0, shown).map((w) => <StNotice key={w} tone="warning">{w}</StNotice>)}
      {rest > 0 && <StNotice tone="warning">另有 {rest} 条提示</StNotice>}
    </div>
  );
}
