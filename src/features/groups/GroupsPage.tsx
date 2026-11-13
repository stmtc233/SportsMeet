import { useState } from "react";
import { IconAdd, IconDelete, IconDown, IconEdit, IconUp, StButton, StCard, StDialog, StEmpty, StIconButton, StListItem, StPage, StTextField, StToolbar, iconStroke } from "../../design";
import type { GroupView } from "../../mygo";
import { DeleteDialog } from "../meets/DeleteDialog";
import { GroupsController, useGroupSuggestions, useGroups } from "./controller";
import { GroupOrganizeDialog } from "./GroupOrganizeDialog";

export function GroupsPage({ meetId }: { meetId: string }) {
  const { data: groups } = useGroups(meetId);
  const { data: suggestions } = useGroupSuggestions(meetId);
  const [editing, setEditing] = useState<{ group?: GroupView } | null>(null);
  const [deleting, setDeleting] = useState<GroupView | null>(null);
  const [organizing, setOrganizing] = useState(false);
  const [moving, setMoving] = useState(false);

  const move = async (index: number, by: number) => {
    if (!groups) return;
    const ids = groups.map((g) => g.id);
    [ids[index], ids[index + by]] = [ids[index + by]!, ids[index]!];
    setMoving(true);
    await GroupsController.reorder(meetId, ids);
    setMoving(false);
  };

  return (
    <StPage scrollKey={`groups:${meetId}`}
      toolbar={
        <StToolbar title="组别" subtitle={groups ? `${groups.length} 个` : ""}>
          <StButton disabled={!groups?.length || !suggestions?.length} onClick={() => setOrganizing(true)}>整理归属</StButton>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加组别</StButton>
        </StToolbar>
      }>
      <div className="narrow">
        <StCard padded={false}>
          {groups?.length === 0 && (
            <StEmpty action={<StButton variant="primary" onClick={() => setEditing({})}>添加组别</StButton>}>还没有组别</StEmpty>
          )}
          {(groups ?? []).map((g, i) => (
            <StListItem key={g.id} title={g.name} subtitle={`${g.events} 个项目 · ${g.units} 个单位 · ${g.athletes} 名运动员`}
              trailing={
                <>
                  <StIconButton label="上移" disabled={moving || i === 0} onClick={() => move(i, -1)}>
                    <IconUp size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton label="下移" disabled={moving || i === groups!.length - 1} onClick={() => move(i, 1)}>
                    <IconDown size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton label="编辑" onClick={() => setEditing({ group: g })}>
                    <IconEdit size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton label="删除" onClick={() => setDeleting(g)}>
                    <IconDelete size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                </>
              } />
          ))}
        </StCard>
      </div>
      {editing && <GroupDialog key={editing.group?.id ?? "new"} meetId={meetId} group={editing.group} onClose={() => setEditing(null)} />}
      {organizing && <GroupOrganizeDialog meetId={meetId} onClose={() => setOrganizing(false)} />}
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.name ?? ""}」？`} onClose={() => setDeleting(null)}
        onDelete={() => GroupsController.remove(meetId, deleting!.id)}>
        {deleting && usage(deleting)}
      </DeleteDialog>
    </StPage>
  );
}

function usage(g: GroupView) {
  const parts = [g.events && `${g.events} 个项目`, g.units && `${g.units} 个单位`, g.athletes && `${g.athletes} 名运动员`].filter(Boolean);
  return parts.length ? `${parts.join("、")}将改为未分组，成绩不变。` : "该组别没有被使用。";
}

function GroupDialog({ meetId, group, onClose }: { meetId: string; group?: GroupView; onClose: () => void }) {
  const [name, setName] = useState(group?.name ?? "");
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    const ok = await GroupsController.save(meetId, group?.id ?? "", { name });
    setBusy(false);
    if (ok) onClose();
  };
  return (
    <StDialog open title={group ? "编辑组别" : "添加组别"} size="small" onClose={() => !busy && onClose()} onSubmit={save}
      actions={
        <>
          <StButton disabled={busy} onClick={onClose}>取消</StButton>
          <StButton variant="primary" loading={busy} type="submit">保存</StButton>
        </>
      }>
      <StTextField label="名称" placeholder="初一" autoFocus width="fill" value={name} onChange={setName} disabled={busy} />
    </StDialog>
  );
}
