import { useState } from "react";
import { StButton, StDialog, StSelect, StTable } from "../../design";
import type { GroupSuggestion } from "../../mygo";
import { GroupsController, useGroupSuggestions, useGroups } from "./controller";

const keyOf = (r: GroupSuggestion) => `${r.kind}:${r.id}`;

/** Units and athletes without a group, each with the group Go suggests. */
export function GroupOrganizeDialog({ meetId, onClose }: { meetId: string; onClose: () => void }) {
  const { data: groups } = useGroups(meetId);
  const { data: rows } = useGroupSuggestions(meetId);
  const [choices, setChoices] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  // Only one candidate: picked already; several: the user chooses.
  const chosen = (r: GroupSuggestion) => choices[keyOf(r)] ?? (r.candidates.length === 1 ? r.candidates[0]! : "");
  const picked = (rows ?? []).filter((r) => chosen(r));
  const options = [{ value: "", label: "暂不调整" }, ...(groups ?? []).map((g) => ({ value: g.id, label: g.name }))];

  const save = async () => {
    setBusy(true);
    const ok = await GroupsController.assign(meetId, picked.map((r) => ({ kind: r.kind, id: r.id, groupIds: [chosen(r)] })));
    setBusy(false);
    if (ok) onClose();
  };

  return (
    <StDialog open title="整理归属" size="large" onClose={() => !busy && onClose()}
      actions={
        <>
          <StButton disabled={busy} onClick={onClose}>取消</StButton>
          <StButton variant="primary" loading={busy} disabled={!picked.length} onClick={save}>
            {picked.length ? `保存 ${picked.length} 项` : "保存"}
          </StButton>
        </>
      }>
      <div className="preview-table" inert={busy}>
        <StTable<GroupSuggestion> rows={rows ?? []} loading={!rows} rowKey={keyOf} empty="都已有组别" columns={[
          { key: "name", title: "名称", width: "minmax(120px, 1fr)", render: (r) => r.name },
          { key: "kind", title: "类型", width: "88px", render: (r) => (r.kind === "unit" ? "参赛单位" : "运动员") },
          { key: "group", title: "组别", width: "160px", render: (r) => (
            <StSelect aria-label={`${r.name}的组别`} width="fill" size="small" value={chosen(r)} options={options}
              onChange={(id) => setChoices((old) => ({ ...old, [keyOf(r)]: id }))} />
          ) },
        ]} />
      </div>
    </StDialog>
  );
}
