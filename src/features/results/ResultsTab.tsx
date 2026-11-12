import { useCallback, useMemo, useState } from "react";
import { useLeaveGuard } from "../../app/nav";
import { StEmpty, StSection, StSegmented, StTable, StTag } from "../../design";
import { statusLabels } from "../../lib/labels";
import type { EntryView, EventDetail, Standing } from "../../mygo";
import { HeatResults } from "./HeatResults";

/** Results by round: each heat is typed and saved on its own; the round's ranking follows. */
export function ResultsTab({ detail }: { detail: EventDetail }) {
  const [roundId, setRoundId] = useState(detail.rounds[0]?.id ?? "");
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const entries = useMemo(() => new Map<string, EntryView>(detail.entries.map((e) => [e.competitorId, e])), [detail.entries]);
  const onDirty = useCallback((id: string, v: boolean) => setDirty((old) => (Boolean(old[id]) === v ? old : { ...old, [id]: v })), []);
  useLeaveGuard(Object.values(dirty).some(Boolean));
  const round = detail.rounds.find((r) => r.id === roundId) ?? detail.rounds[0];
  if (!round) return <StEmpty>没有轮次</StEmpty>;
  return (
    <div className="fill">
      <div className="subbar">
        {detail.rounds.length > 1 && <StSegmented size="small" value={round.id} onChange={setRoundId} options={detail.rounds.map((r) => ({ value: r.id, label: r.label }))} />}
        <span className="subbar__spacer" />
        {round.heats.length > 0 && <StTag tone={round.complete ? "success" : "neutral"}>{round.complete ? "已录完" : "录入中"}</StTag>}
      </div>
      <div className="split__main">
        {round.heats.length === 0 && <StEmpty>先在「编排」里分组</StEmpty>}
        {round.heats.map((h) => (
          <HeatResults key={h.id} detail={detail} heat={h} entries={entries} onDirty={onDirty} />
        ))}
        {round.standings.length > 0 && (
          <StSection title={`${round.label}排名`} padded={false}>
            <StTable<Standing> dense rows={round.standings} rowKey={(s) => s.competitorId}
              columns={[
                { key: "rank", title: "名次", width: "64px", align: "end", render: (s) => s.rank ?? "" },
                { key: "name", title: detail.event.kind === "relay" ? "队伍" : "姓名", render: (s) => <span className="cell-strong">{entries.get(s.competitorId)?.name}</span> },
                { key: "unit", title: "单位", render: (s) => entries.get(s.competitorId)?.unitName },
                { key: "heat", title: "组", width: "96px", render: (s) => s.heatLabel },
                { key: "mark", title: "成绩", width: "112px", align: "end", render: (s) => (s.status === "OK" ? s.markText : statusLabels[s.status]) },
                { key: "q", title: "晋级", width: "64px", render: (s) => s.qualify && <StTag tone="success">{s.qualify}</StTag> },
              ]} />
          </StSection>
        )}
      </div>
    </div>
  );
}
