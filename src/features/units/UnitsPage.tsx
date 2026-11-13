import { useEffect, useState } from "react";
import { IconAdd, IconDelete, StButton, StDialog, StEmpty, StIconButton, StPage, StTable, StTag, StTextField, StToolbar, iconStroke } from "../../design";
import type { Unit, UnitInput } from "../../mygo";
import { GroupsController, useGroups } from "../groups/controller";
import { GroupChecks, GroupMenu } from "../groups/GroupFields";
import { DeleteDialog } from "../meets/DeleteDialog";
import { UnitsController, useUnits } from "./controller";

export function UnitsPage({ meetId }: { meetId: string }) {
  const { data: units } = useUnits(meetId);
  const { data: groups } = useGroups(meetId);
  const [selected, setSelected] = useState(new Set<string>());
  const [editing, setEditing] = useState<{ unit?: Unit } | null>(null);
  const [deleting, setDeleting] = useState<Unit | null>(null);
  const picked = (units ?? []).filter((u) => selected.has(u.id));
  const groupNames = (u: Unit) => (groups ?? []).filter((g) => u.groupIds?.includes(g.id));

  return (
    <StPage flush
      toolbar={
        <StToolbar title="参赛单位" subtitle={units ? `${units.length} 个` : ""}>
          <GroupMenu meetId={meetId} count={picked.length}
            onPick={(id) => GroupsController.assign(meetId, picked.map((u) => ({ kind: "unit", id: u.id, groupIds: id ? [id] : [] })))} />
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加单位</StButton>
        </StToolbar>
      }>
      <div className="table-card table-card--top">
        <StTable<Unit> fill rows={units ?? []} loading={!units} scrollKey={`units:${meetId}`} rowKey={(u) => u.id} rowLabel={(u) => u.name} selected={selected} onSelect={setSelected}
          onRowClick={(u) => setEditing({ unit: u })}
          empty={<StEmpty action={<StButton variant="primary" onClick={() => setEditing({})}>添加单位</StButton>}>还没有参赛单位</StEmpty>}
          columns={[
            { key: "name", title: "名称", width: "minmax(160px, 2fr)", render: (u) => <span className="cell-strong">{u.name}</span> },
            { key: "short", title: "简称", width: "minmax(96px, 1fr)", render: (u) => u.shortName },
            { key: "groups", title: "组别", width: "minmax(120px, 1.5fr)", render: (u) => (
              <span className="cell-tags">{groupNames(u).map((g) => <StTag key={g.id}>{g.name}</StTag>)}</span>
            ) },
            { key: "delete", title: "", width: "48px", align: "end", render: (u) => (
              <span onClick={(e) => e.stopPropagation()}>
                <StIconButton size="small" label="删除" onClick={() => setDeleting(u)}>
                  <IconDelete size={16} strokeWidth={iconStroke} />
                </StIconButton>
              </span>
            ) },
          ]} />
      </div>
      <UnitDialog meetId={meetId} open={editing !== null} unit={editing?.unit} onClose={() => setEditing(null)} />
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.name ?? ""}」？`} onClose={() => setDeleting(null)} onDelete={() => UnitsController.remove(meetId, deleting!.id)}>
        该单位的运动员会保留，但不再属于任何单位。
      </DeleteDialog>
    </StPage>
  );
}

function UnitDialog({ meetId, open, unit, onClose }: { meetId: string; open: boolean; unit?: Unit; onClose: () => void }) {
  const [form, setForm] = useState<Required<UnitInput>>({ name: "", shortName: "", groupIds: [] });
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm({ name: unit?.name ?? "", shortName: unit?.shortName ?? "", groupIds: unit?.groupIds ?? [] });
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
      <GroupChecks meetId={meetId} value={form.groupIds} onChange={(groupIds) => setForm((f) => ({ ...f, groupIds }))} />
    </StDialog>
  );
}
