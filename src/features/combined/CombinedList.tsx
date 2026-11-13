import { useState } from "react";
import { useNav } from "../../app/nav";
import { IconBack, StButton, StEmpty, StIconButton, StPage, StTable, StToolbar, iconStroke } from "../../design";
import type { CombinedDetail, CombinedRowSummary } from "../../mygo";
import { GroupsController, useGroups } from "../groups/controller";
import { GroupFilter, GroupMenu, inGroup, useGroupFilter } from "../groups/GroupFields";
import { DeleteDialog } from "../meets/DeleteDialog";
import { CombinedDialog } from "./CombinedDialog";
import { CombinedController, useCombinedList } from "./controller";

export function CombinedList({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data: allRows } = useCombinedList(meetId);
  const { data: groups } = useGroups(meetId);
  const [group, setGroup] = useGroupFilter(meetId, "combineds");
  const rows = allRows?.filter((r) => inGroup(group, r.combined.groupId));
  const [selected, setSelected] = useState(new Set<string>());
  const picked = (rows ?? []).filter((r) => selected.has(r.combined.id)).map((r) => r.combined.id);
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<CombinedDetail | null>(null);
  const askDelete = async (id: string) => { const d = await CombinedController.get(id); if (d) setDeleting(d); };
  return <StPage flush toolbar={<StToolbar title="全能" leading={<StIconButton label="返回项目列表" onClick={() => go({ page: "meet", meetId, tab: "events" })}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton>}>
    <GroupMenu meetId={meetId} count={picked.length} onPick={(id) => GroupsController.move(meetId, id, [], picked)} />
    <StButton variant="primary" onClick={() => setAdding(true)}>添加全能</StButton>
  </StToolbar>}>
    {groups?.length ? <div className="filters"><GroupFilter meetId={meetId} value={group} onChange={setGroup} /></div> : null}
    <div className={groups?.length ? "table-card" : "table-card table-card--top"}><StTable<CombinedRowSummary> fill rows={rows ?? []} selected={selected} onSelect={setSelected} loading={!rows} scrollKey={`combineds:${meetId}`} rowKey={(r) => r.combined.id} onRowClick={(r) => go({ page: "combined", meetId, combinedId: r.combined.id })}
      empty={<StEmpty>还没有全能</StEmpty>} columns={[
        { key: "name", title: "全能", render: (r) => <span className="cell-strong">{r.combined.name}</span> },
        { key: "gender", title: "性别", render: (r) => r.combined.gender },
        { key: "group", title: "组别", render: (r) => r.combined.groupLabel },
        { key: "components", title: "分项", align: "end", render: (r) => r.components },
        { key: "entries", title: "报名", align: "end", render: (r) => r.entries },
        { key: "delete", title: "", render: (r) => <span onClick={(e) => e.stopPropagation()}><StButton size="small" onClick={() => askDelete(r.combined.id)}>删除</StButton></span> },
      ]} /></div>
    {adding && <CombinedDialog initialGroupId={group === "all" ? "" : group} meetId={meetId} onClose={() => setAdding(false)} />}
    <DeleteDialog open={deleting !== null} title="删除全能？" onClose={() => setDeleting(null)} onDelete={() => CombinedController.remove(deleting!.combined.id)}>
      将删除 {deleting?.components.length ?? 0} 个分项、{deleting?.heats ?? 0} 组与 {deleting?.results ?? 0} 条成绩，无法撤销。
    </DeleteDialog>
  </StPage>;
}
