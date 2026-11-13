import { useRef } from "react";
import { StButton, StCard, StEmpty, StNotice, StRow, StSegmented, StSelect, StStack, StTable } from "../../design";
import type { AuditRow } from "../../mygo";
import { useViewState } from "../../lib/viewState";
import { useMeets } from "../meets/data";
import { useAuditLog } from "./controller";

type SourceFilter = "all" | "ui" | "mcp" | "system";
const sources: { value: SourceFilter; label: string }[] = [
  { value: "all", label: "全部" }, { value: "ui", label: "界面" }, { value: "mcp", label: "MCP" }, { value: "system", label: "系统" },
];

/** 操作日志: read-only, newest first, filtered by meet and source. */
export function AuditPanel() {
  const [savedMeet, setMeetId] = useViewState("audit:meet", "all");
  const [source, setSource] = useViewState<SourceFilter>("audit:source", "all");
  const { data: meets } = useMeets();
  const meetId = meets && !meets.some((m) => m.id === savedMeet) ? "all" : savedMeet;
  const { log, error, loading, more } = useAuditLog(meetId === "all" ? "" : meetId, source === "all" ? "" : source);
  // While another filter loads, the previous rows stay (dimmed): the table does not collapse and regrow.
  const lastLog = useRef(log);
  if (log) lastLog.current = log;
  const shown = log ?? lastLog.current;
  const cell = (text: string) => <span title={text}>{text}</span>;
  return (
    <StStack gap={16}>
      <StRow>
        <StSelect label="赛事" value={meetId} width="auto" onChange={setMeetId}
          options={[{ value: "all", label: "全部赛事" }, ...(meets ?? []).map((m) => ({ value: m.id, label: m.name }))]} />
        <StSegmented label="来源" value={source} options={sources} onChange={setSource} />
      </StRow>
      {error && <StNotice tone="danger">{error}</StNotice>}
      <div className="stale-aware" data-stale={!log && shown ? true : undefined}><StCard padded={false}>
        <StTable<AuditRow> dense rows={shown?.rows ?? []} rowKey={(r) => String(r.id)} loading={!log}
          empty={<StEmpty>没有记录</StEmpty>}
          columns={[
            { key: "at", title: "时间", width: "136px", render: (r) => r.atText },
            { key: "source", title: "来源", width: "56px", render: (r) => r.sourceText },
            { key: "action", title: "操作", width: "minmax(96px, 1fr)", render: (r) => cell(r.actionText) },
            { key: "message", title: "内容", width: "minmax(0, 2fr)", render: (r) => cell(r.message) },
            { key: "meet", title: "赛事", width: "minmax(0, 1fr)", render: (r) => cell(r.meetName) },
          ]} />
      </StCard></div>
      {log && log.rows.length > 0 && <StRow><StButton disabled={!log.next} loading={loading} onClick={() => void more()}>加载更多</StButton></StRow>}
    </StStack>
  );
}
