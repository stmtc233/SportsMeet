import { useEffect, useState } from "react";
import { StButton, StDialog, StTextField } from "../../design";
import type { Unit, UnitInput } from "../../mygo";
import { GroupChecks } from "../groups/GroupFields";
import { UnitsController } from "./controller";

/** Adds a unit (unit undefined; in groupId when given) or edits one. */
export function UnitDialog({ meetId, open, unit, groupId, onClose }: { meetId: string; open: boolean; unit?: Unit; groupId?: string; onClose: () => void }) {
  const [form, setForm] = useState<Required<UnitInput>>({ name: "", shortName: "", groupIds: [] });
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm({ name: unit?.name ?? "", shortName: unit?.shortName ?? "", groupIds: unit ? unit.groupIds ?? [] : groupId ? [groupId] : [] });
  }, [open, unit, groupId]);
  const save = async () => {
    setBusy(true);
    const ok = await UnitsController.save(meetId, unit?.id ?? "", form);
    setBusy(false);
    if (ok) onClose();
  };
  return (
    <StDialog open={open} title={unit ? "编辑单位" : "添加单位"} size="small" onClose={onClose} onSubmit={save}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
        </>
      }>
      <StTextField label="名称" value={form.name} onChange={(name) => setForm((f) => ({ ...f, name }))} width="fill" autoFocus />
      <StTextField label="简称" hint="用于秩序册" value={form.shortName} onChange={(shortName) => setForm((f) => ({ ...f, shortName }))} width="fill" />
      <GroupChecks meetId={meetId} value={form.groupIds} onChange={(groupIds) => setForm((f) => ({ ...f, groupIds }))} />
    </StDialog>
  );
}
