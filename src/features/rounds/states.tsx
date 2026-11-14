import { StTag } from "../../design";
import type { RoundRow } from "../../mygo";

/** Where a round stands for 编排. */
export function ArrangeState({ row: r }: { row: RoundRow }) {
  switch (r.arrange) {
    case "empty":
      return <StTag>无报名</StTag>;
    case "todo":
      return <StTag>未编排</StTag>;
    case "waiting":
      return <StTag>等{r.prevLabel}</StTag>;
    case "ready":
      return <StTag tone="info">可从{r.prevLabel}晋级</StTag>;
  }
  return (
    <span className="round-state">
      <StTag tone="success">已编排</StTag>
      {r.canArrange && <StTag tone="warning">未排 {r.unplaced}</StTag>}
    </span>
  );
}

/** Where a round stands for 成绩; a round without heats is not arranged yet. */
export function ResultsState({ row: r }: { row: RoundRow }) {
  switch (r.results) {
    case "":
      return r.canArrange ? <StTag tone="info">可编排</StTag> : <StTag>未编排</StTag>;
    case "none":
      return <StTag>未开始</StTag>;
    case "doing":
      return <StTag tone="warning">录入中 {r.heatsDone}/{r.heats}</StTag>;
    case "ties":
      return <StTag tone="danger">待裁定并列</StTag>;
    case "advance":
      return <StTag tone="info">可编排{r.nextLabel}</StTag>;
  }
  return <StTag tone="success">已录完</StTag>;
}
