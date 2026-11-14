import { useState } from "react";
import { StButton, StCheckbox, StConfirmButton, StDialog, StEmpty, StRow, StTable, StTag } from "../../design";
import type { EventDetail, TeamView } from "../../mygo";
import { EventsController, useEntryLimitWarnings } from "../events/controller";
import { LimitNotice } from "./LimitNotice";
import { TeamDialog } from "./TeamDialog";

export function AddTeamsDialog({ open, detail, meetId, onClose }: { open: boolean; detail: EventDetail; meetId: string; onClose: () => void }) {
  const [editing, setEditing] = useState<TeamView | null | undefined>();
  const [selected, setSelected] = useState(new Set<string>());
  const [busy, setBusy] = useState(false);
  const pending = detail.teams.filter((t) => !t.entered && selected.has(t.id));
  const warnings = useEntryLimitWarnings(detail.event.id, open ? pending.map((t) => t.id) : []);
  const add = async () => {
    if (busy) return;
    setBusy(true);
    const ok = await EventsController.addEntries(detail.event.id, pending.map((t) => ({ teamId: t.id })));
    setBusy(false);
    if (ok) { setSelected(new Set()); onClose(); }
  };
  const remove = async (id: string) => {
    setBusy(true);
    await EventsController.deleteTeam(detail.event.id, id);
    setBusy(false);
  };
  return <>
    <StDialog open={open && editing === undefined} title={detail.event.kind === "relay" ? "接力队伍" : "队伍"} size="large" onClose={() => !busy && onClose()}
      actions={<><StButton disabled={busy} onClick={onClose}>关闭</StButton><StButton variant="primary" disabled={!pending.length} loading={busy} onClick={add}>报名{pending.length ? ` ${pending.length} 队` : ""}</StButton></>}>
      <StRow><StButton disabled={busy} onClick={() => setEditing(null)}>添加队伍</StButton></StRow>
      <LimitNotice warnings={warnings} />
      <div className="preview-table">
        <StTable<TeamView> rows={detail.teams} rowKey={(t) => t.id} dense empty={<StEmpty>还没有队伍</StEmpty>}
          columns={[
            { key: "selected", title: "报名", width: "88px", render: (t) => t.entered ? <StTag>已报</StTag> : <StCheckbox checked={selected.has(t.id)} disabled={busy} label="报名" onChange={(value) => setSelected((old) => { const next = new Set(old); if (value) next.add(t.id); else next.delete(t.id); return next; })} /> },
            { key: "name", title: "队伍", width: "minmax(100px, 1fr)", render: (t) => t.name },
            { key: "unit", title: "单位", width: "minmax(80px, 1fr)", render: (t) => t.unitName },
            { key: "members", title: detail.event.kind === "relay" ? "棒次" : "成员", width: "minmax(140px, 2fr)", render: (t) => <>{t.memberNames.join(detail.event.kind === "relay" ? " → " : "、")}{t.groupMismatch && <StTag tone="warning">跨组成员</StTag>}{t.sizeWarning && <StTag tone="warning">{t.sizeWarning}</StTag>}</> },
            { key: "actions", title: "", width: "208px", render: (t) => <StRow wrap={false}><StButton size="small" disabled={busy} onClick={() => setEditing(t)}>编辑</StButton><StConfirmButton size="small" disabled={busy || t.placed} onConfirm={() => remove(t.id)}>删除</StConfirmButton></StRow> },
          ]} />
      </div>
    </StDialog>
    {open && editing !== undefined && <TeamDialog detail={detail} meetId={meetId} team={editing} onClose={() => setEditing(undefined)} />}
  </>;
}
