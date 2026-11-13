import { useEffect, useState } from "react";
import { IconAdd, IconDelete, IconEdit, StButton, StCard, StDialog, StEmpty, StIconButton, StListItem, StPage, StTextField, StToolbar, iconStroke } from "../../design";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Units, type Unit, type UnitInput } from "../../mygo";
import { DeleteDialog } from "../meets/DeleteDialog";

export function useUnits(meetId: string) {
  return useData(() => Units.list(meetId), [meetId], (c) => c.meetId === meetId && (c.kind === "units" || c.kind === "athletes" || c.kind === "meets"), `units:${meetId}`);
}

export const UnitsController = {
  save: (meetId: string, id: string, input: UnitInput) => act(() => Units.save(meetId, id, input), "已保存"),
  remove: (meetId: string, id: string) => act(() => Units.delete(meetId, id), "已删除"),
};

export function UnitsPage({ meetId }: { meetId: string }) {
  const { data: units } = useUnits(meetId);
  const [editing, setEditing] = useState<{ unit?: Unit } | null>(null);
  const [deleting, setDeleting] = useState<Unit | null>(null);
  return (
    <StPage scrollKey={`units:${meetId}`}
      toolbar={
        <StToolbar title="参赛单位" subtitle={units ? `${units.length} 个` : ""}>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加单位</StButton>
        </StToolbar>
      }>
      <div className="narrow">
        <StCard padded={false}>
          {units?.length === 0 && <StEmpty>还没有参赛单位</StEmpty>}
          {(units ?? []).map((u) => (
            <StListItem key={u.id} title={u.name} subtitle={u.shortName}
              trailing={
                <>
                  <StIconButton label="编辑" onClick={() => setEditing({ unit: u })}>
                    <IconEdit size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton label="删除" onClick={() => setDeleting(u)}>
                    <IconDelete size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                </>
              } />
          ))}
        </StCard>
      </div>
      <UnitDialog meetId={meetId} open={editing !== null} unit={editing?.unit} onClose={() => setEditing(null)} />
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.name ?? ""}」？`} onClose={() => setDeleting(null)} onDelete={() => UnitsController.remove(meetId, deleting!.id)}>
        该单位的运动员会保留，但不再属于任何单位。
      </DeleteDialog>
    </StPage>
  );
}

function UnitDialog({ meetId, open, unit, onClose }: { meetId: string; open: boolean; unit?: Unit; onClose: () => void }) {
  const [form, setForm] = useState<Required<UnitInput>>({ name: "", shortName: "" });
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm({ name: unit?.name ?? "", shortName: unit?.shortName ?? "" });
  }, [open, unit]);
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
    </StDialog>
  );
}
