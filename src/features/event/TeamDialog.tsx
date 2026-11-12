import { useState } from "react";
import { IconDelete, IconDown, IconUp, StButton, StDialog, StIconButton, StNotice, StRow, StSelect, StStack, StTextField, iconStroke } from "../../design";
import type { EventDetail, TeamView } from "../../mygo";
import { useAthletes } from "../athletes/controller";
import { EventsController, useTeamSizeWarning } from "../events/controller";
import { useUnits } from "../units/UnitsPage";

export function TeamDialog({ detail, meetId, team, onClose }: { detail: EventDetail; meetId: string; team: TeamView | null; onClose: () => void }) {
  const { data: athletes } = useAthletes(meetId);
  const { data: units } = useUnits(meetId);
  const [name, setName] = useState(team?.name ?? "");
  const [unitId, setUnitId] = useState(team?.unitId ?? "");
  const [members, setMembers] = useState(team?.memberIds ?? []);
  const sizeWarning = useTeamSizeWarning(detail.event, members.length);
  const [busy, setBusy] = useState(false);
  const locked = Boolean(team?.keepReason);
  const otherMembers = new Set(detail.teams.filter((t) => t.id !== team?.id).flatMap((t) => t.memberIds));
  const mismatch = (athletes ?? []).some(({ athlete: a }) => members.includes(a.id) && a.gender !== "" && detail.event.gender !== "混合" && a.gender !== detail.event.gender);
  const save = async () => {
    if (busy) return;
    setBusy(true);
    const ok = await EventsController.saveTeam(detail.event.id, team?.id ?? "", { name, unitId, memberIds: members });
    setBusy(false);
    if (ok) onClose();
  };
  const move = (i: number, delta: number) => setMembers((old) => {
    const next = [...old];
    [next[i], next[i + delta]] = [next[i + delta]!, next[i]!];
    return next;
  });
  return (
    <StDialog open title={team ? "编辑队伍" : "添加队伍"} onClose={() => !busy && onClose()} onSubmit={save}
      actions={<><StButton disabled={busy} onClick={onClose}>取消</StButton><StButton variant="primary" type="submit" loading={busy} disabled={!name.trim()}>保存</StButton></>}>
      <StTextField label="队名" value={name} onChange={setName} autoFocus disabled={busy} />
      <StSelect label="参赛单位" value={unitId} onChange={setUnitId} disabled={busy}
        options={[{ value: "", label: "未指定" }, ...(units ?? []).map((u) => ({ value: u.id, label: u.name }))]} />
      {locked && <StNotice>已有比赛记录，棒次已锁定</StNotice>}
      {sizeWarning && <StNotice tone="warning">{sizeWarning}</StNotice>}
      {mismatch && <StNotice tone="warning">成员性别与项目不符</StNotice>}
      <StStack>
        {members.map((id, i) => (
          <StRow key={i} wrap={false}>
            <StSelect label={`第 ${i + 1} 棒`} width="fill" value={id} disabled={busy || locked}
              options={(athletes ?? []).filter(({ athlete: a }) => a.id === id || (!members.includes(a.id) && !otherMembers.has(a.id))).map(({ athlete: a }) => ({ value: a.id, label: `${a.name}${a.bib ? ` · ${a.bib}` : ""}` }))}
              onChange={(value) => setMembers((old) => old.map((m, n) => n === i ? value : m))} />
            <StIconButton label={`上移第 ${i + 1} 棒`} disabled={busy || locked || i === 0} onClick={() => move(i, -1)}><IconUp size={16} strokeWidth={iconStroke} /></StIconButton>
            <StIconButton label={`下移第 ${i + 1} 棒`} disabled={busy || locked || i === members.length - 1} onClick={() => move(i, 1)}><IconDown size={16} strokeWidth={iconStroke} /></StIconButton>
            <StIconButton label={`移除第 ${i + 1} 棒`} disabled={busy || locked} onClick={() => setMembers((old) => old.filter((_, n) => n !== i))}><IconDelete size={16} strokeWidth={iconStroke} /></StIconButton>
          </StRow>
        ))}
        <StSelect label="添加下一棒" value="" placeholder="选择运动员" disabled={busy || locked} width="fill"
          options={(athletes ?? []).filter(({ athlete: a }) => !members.includes(a.id) && !otherMembers.has(a.id)).map(({ athlete: a }) => ({ value: a.id, label: `${a.name}${a.bib ? ` · ${a.bib}` : ""}` }))}
          onChange={(id) => setMembers((old) => [...old, id])} />
      </StStack>
    </StDialog>
  );
}
