import { useCallback, useMemo, useRef, useState } from "react";
import { useLeaveGuard } from "../../app/nav";
import { StEmpty, StMuted, StSection, StSegmented, StTable, StTag } from "../../design";
import { statusLabels } from "../../lib/labels";
import { useScrollMemory, useViewState } from "../../lib/viewState";
import type { EntryView, EventDetail, Standing } from "../../mygo";
import { HeatResults } from "./HeatResults";

/** Results by round: each heat is typed and saved on its own; the round's ranking follows. */
export function ResultsTab({ detail }: { detail: EventDetail }) {
  // Shared with the arrange page: switching tabs keeps the round.
  const [roundId, setRoundId] = useViewState(`event:round:${detail.event.id}`, detail.rounds[0]?.id ?? "");
  const [visited, setVisited] = useState(() => [roundId]);
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const entries = useMemo(() => new Map<string, EntryView>(detail.entries.map((e) => [e.competitorId, e])), [detail.entries]);
  const onDirty = useCallback((id: string, v: boolean) => setDirty((old) => (Boolean(old[id]) === v ? old : { ...old, [id]: v })), []);
  useLeaveGuard(Object.values(dirty).some(Boolean));
  const round = detail.rounds.find((r) => r.id === roundId) ?? detail.rounds[0];
  const box = useRef<HTMLDivElement>(null);
  useScrollMemory(box, `results:${detail.event.id}:${round?.id}`);
  if (!round) return <StEmpty>没有轮次</StEmpty>;
  const roundDirty = round.heats.some((heat) => dirty[heat.id]);
  const selectRound = (id: string) => { setVisited((old) => old.includes(id) ? old : [...old, id]); setRoundId(id); };
  return (
    <div className="fill view-enter">
      <div className="subbar">
        {detail.rounds.length > 1 && <StSegmented aria-label="轮次" size="small" value={round.id} onChange={selectRound} options={detail.rounds.map((r) => ({ value: r.id, label: r.label }))} />}
        <span className="subbar__spacer" />
        {round.heats.length > 0 && <StTag tone={round.complete && !roundDirty ? "success" : "neutral"}>{round.complete && !roundDirty ? "已录完" : "录入中"}</StTag>}
      </div>
      <div ref={box} className="split__main">
        {round.heats.length === 0 && <StEmpty>先在「编排」里分组</StEmpty>}
        {detail.rounds.filter((item) => visited.includes(item.id) || item.id === round.id).map((item) => <div key={item.id} className="result-round" hidden={item.id !== round.id}>
          {item.heats.map((h) => <HeatResults key={h.id} detail={detail} heat={h} entries={entries} onDirty={onDirty} />)}
        </div>)}
        {round.standings.length > 0 && (
          <StSection title={`${round.label}${roundDirty ? "已保存排名" : round.complete ? "排名" : "暂定排名"}`} padded={false}>
            <StTable<Standing> dense rows={round.standings} rowKey={(s) => s.competitorId}
              columns={[
                { key: "rank", title: "名次", width: "64px", align: "end", render: (s) => <span className="cell-strong">{s.rank ?? ""}</span> },
                { key: "name", title: detail.rules.team ? "队伍" : "姓名", render: (s) => <span className="cell-strong">{entries.get(s.competitorId)?.name}</span> },
                { key: "unit", title: "单位", render: (s) => entries.get(s.competitorId)?.unitName },
                { key: "heat", title: "组", width: "96px", render: (s) => s.heatLabel },
                { key: "mark", title: "成绩", width: "112px", align: "end", render: (s) => (s.status === "OK" ? <span className="result-mark">{s.markText}</span> : <StMuted>{s.markText || statusLabels[s.status]}</StMuted>) },
                { key: "q", title: "晋级", width: "64px", render: (s) => s.qualify && <StTag tone="success">{s.qualify}</StTag> },
              ]} />
          </StSection>
        )}
      </div>
    </div>
  );
}
