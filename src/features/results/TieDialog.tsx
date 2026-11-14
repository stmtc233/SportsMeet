import { useMemo, useState } from "react";
import { StButton, StDialog, StMuted, StSelect, StTable, StTag } from "../../design";
import type { EntryView, EventDetail, RoundView } from "../../mygo";
import { ResultsController } from "./controller";
import { changedPlaces, storedPlaces, tieRows, type TieRow } from "./ties";

/** Orders tied competitors of a round; equal places stay tied. Places and Q / q come back from Go. */
export function TieDialog({ detail, round, entries, onClose }: { detail: EventDetail; round: RoundView; entries: Map<string, EntryView>; onClose: () => void }) {
  const rows = useMemo(() => tieRows(round), [round]);
  const stored = useMemo(() => storedPlaces(round), [round]);
  const standing = useMemo(() => new Map(round.standings.map((s) => [s.competitorId, s])), [round.standings]);
  const [chosen, setChosen] = useState(stored);
  const [busy, setBusy] = useState(false);
  const places = changedPlaces(round, stored, chosen);
  const save = async () => {
    setBusy(true);
    const saved = await ResultsController.ruleTies({ roundId: round.id, places });
    setBusy(false);
    if (saved !== undefined) onClose();
  };
  const close = () => { if (!busy) onClose(); };
  return (
    <StDialog open title="裁定并列" size="large" onClose={close}
      actions={
        <>
          <StButton disabled={busy} onClick={close}>取消</StButton>
          <StButton variant="primary" loading={busy} disabled={!places.length} onClick={save}>保存</StButton>
        </>
      }>
      <div className="preview-table" inert={busy}>
        <StTable<TieRow> dense rows={rows} rowKey={(r) => r.competitorId} columns={[
          { key: "tie", title: "并列", width: "152px", render: (r) => {
            const tie = round.ties[r.tie]!;
            return r.first && <span className="tie-head">第 {tie.place} 名{tie.extra > 0 && <StTag tone="warning">多 {tie.extra} 人晋级</StTag>}</span>;
          } },
          { key: "name", title: detail.rules.team ? "队伍" : "姓名", width: "minmax(96px, 1fr)", render: (r) => <span className="cell-strong">{entries.get(r.competitorId)?.name}</span> },
          { key: "unit", title: "单位", width: "minmax(80px, 1fr)", render: (r) => entries.get(r.competitorId)?.unitName },
          { key: "heat", title: "组", width: "80px", render: (r) => <StMuted>{standing.get(r.competitorId)?.heatLabel}</StMuted> },
          { key: "mark", title: "成绩", width: "88px", align: "end", render: (r) => <span className="result-mark">{standing.get(r.competitorId)?.markText}</span> },
          { key: "place", title: "名次", width: "112px", render: (r) => {
            const tie = round.ties[r.tie]!;
            const name = entries.get(r.competitorId)?.name ?? "";
            return <StSelect aria-label={`${name}的名次`} width="fill" size="small" value={String(chosen[r.competitorId] ?? tie.place)}
              options={tie.competitorIds.map((_, i) => ({ value: String(tie.place + i), label: `第 ${tie.place + i} 名` }))}
              onChange={(v) => setChosen((old) => ({ ...old, [r.competitorId]: Number(v) }))} />;
          } },
        ]} />
      </div>
    </StDialog>
  );
}
