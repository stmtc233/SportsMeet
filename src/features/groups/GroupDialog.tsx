import { useState } from "react";
import { StButton, StCheckbox, StDialog, StMuted, StStack, StTextField } from "../../design";
import type { GroupView, Unit } from "../../mygo";
import { DeleteDialog } from "../meets/DeleteDialog";
import { useUnits } from "../units/controller";
import { GroupsController } from "./controller";

/** Adds a group (group undefined) or edits one: its name and the units in it. */
export function GroupDialog({ meetId, group, onClose }: { meetId: string; group?: GroupView; onClose: () => void }) {
  const { data: units } = useUnits(meetId);
  const [name, setName] = useState(group?.name ?? "");
  // The units ticked here; null until changed, so units read later still show as they are.
  const [picked, setPicked] = useState<Set<string> | null>(null);
  const [busy, setBusy] = useState(false);
  const inGroup = (u: Unit) => !!group && (u.groupIds ?? []).includes(group.id);
  const ticked = picked ?? new Set((units ?? []).filter(inGroup).map((u) => u.id));

  const save = async () => {
    setBusy(true);
    const changed = (units ?? []).filter((u) => inGroup(u) !== ticked.has(u.id));
    const ok = await GroupsController.save(meetId, group?.id ?? "", { name }, (id) => changed.map((u) => ({
      kind: "unit", id: u.id, groupIds: ticked.has(u.id) ? [...(u.groupIds ?? []), id] : (u.groupIds ?? []).filter((g) => g !== id),
    })));
    setBusy(false);
    if (ok) onClose();
  };
  const toggle = (id: string, on: boolean) => {
    const next = new Set(ticked);
    if (on) next.add(id);
    else next.delete(id);
    setPicked(next);
  };
  return (
    <StDialog open title={group ? "编辑组别" : "添加组别"} onClose={() => !busy && onClose()} onSubmit={save}
      actions={
        <>
          <StButton disabled={busy} onClick={onClose}>取消</StButton>
          <StButton variant="primary" loading={busy} type="submit">保存</StButton>
        </>
      }>
      <StTextField label="名称" placeholder="初一" autoFocus width="fill" value={name} onChange={setName} disabled={busy} />
      {!!units?.length && (
        <StStack gap={8}>
          <StMuted>参赛单位</StMuted>
          <div className="group-units">
            {units.map((u) => <StCheckbox key={u.id} label={u.name} checked={ticked.has(u.id)} disabled={busy} onChange={(on) => toggle(u.id, on)} />)}
          </div>
        </StStack>
      )}
    </StDialog>
  );
}

/** Deleting a group: what uses it turns 未分组. */
export function GroupDeleteDialog({ meetId, group, onClose }: { meetId: string; group: GroupView | null; onClose: () => void }) {
  return (
    <DeleteDialog open={group !== null} title={`删除「${group?.name ?? ""}」？`} onClose={onClose}
      onDelete={() => GroupsController.remove(meetId, group!.id)}>
      {group && usage(group)}
    </DeleteDialog>
  );
}

function usage(g: GroupView) {
  const parts = [g.events && `${g.events} 个项目`, g.units && `${g.units} 个单位`, g.athletes && `${g.athletes} 名运动员`].filter(Boolean);
  return parts.length ? `${parts.join("、")}将改为未分组，成绩不变。` : "该组别没有被使用。";
}
