import { useState } from "react";
import { useNav, type EventTab } from "../../app/nav";
import { IconAdd, IconDelete, IconEdit, StButton, StEmpty, StIconButton, StPage, StTable, StTag, StToolbar, iconStroke } from "../../design";
import { kindLabels } from "../../lib/labels";
import type { EventRow, MeetCounts } from "../../mygo";
import { GroupsController, useGroups } from "../groups/controller";
import { GroupFilter, GroupMenu, inGroup, useGroupFilter } from "../groups/GroupFields";
import { DeleteDialog } from "../meets/DeleteDialog";
import { EventsController, useEvents } from "./controller";
import { EventDialog } from "./EventDialog";

function progress(row: EventRow) {
  if (row.heats === 0) return <StTag>未编排</StTag>;
  if (row.results === 0) return <StTag tone="info">已编排</StTag>;
  if (row.results < row.placed) return <StTag tone="warning">录入中 {row.results}/{row.placed}</StTag>;
  return <StTag tone="success">已完成</StTag>;
}

export function EventsPage({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data: allRows } = useEvents(meetId);
  const { data: groups } = useGroups(meetId);
  const [group, setGroup] = useGroupFilter(meetId, "events");
  const rows = allRows?.filter((r) => !r.event.combinedId && inGroup(group, r.event.groupId));
  const [selected, setSelected] = useState(new Set<string>());
  const picked = (rows ?? []).filter((r) => selected.has(r.event.id)).map((r) => r.event.id);
  const [editing, setEditing] = useState<{ row?: EventRow } | null>(null);
  const [deleting, setDeleting] = useState<{ row: EventRow; counts: MeetCounts } | null>(null);
  const open = (row: EventRow, tab: EventTab) => go({ page: "event", meetId, eventId: row.event.id, tab });
  const askDelete = async (row: EventRow) => {
    const counts = await EventsController.count(row.event.id);
    if (counts) setDeleting({ row, counts });
  };
  return (
    <StPage flush
      toolbar={
        <StToolbar title="项目" subtitle={rows ? `${rows.length} 项` : ""}>
          <GroupMenu meetId={meetId} count={picked.length} onPick={(id) => GroupsController.move(meetId, id, picked, [])} />
          <StButton onClick={() => go({ page: "combineds", meetId })}>全能</StButton>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加项目</StButton>
        </StToolbar>
      }>
      <div className="filters">{!!groups?.length && <GroupFilter meetId={meetId} value={group} onChange={setGroup} />}</div>
      <div className="table-card">
        <StTable<EventRow> fill rows={rows ?? []} selected={selected} onSelect={setSelected} loading={!rows} scrollKey={`events:${meetId}`} rowKey={(r) => r.event.id} rowLabel={(r) => r.event.name} actionColumn="name" onRowClick={(r) => open(r, r.heats === 0 ? "entries" : r.results > 0 ? "results" : "arrange")}
          empty={<StEmpty action={<StButton variant="primary" onClick={() => setEditing({})}>添加项目</StButton>}>还没有项目</StEmpty>}
          columns={[
            { key: "name", title: "项目", width: "minmax(160px, 2fr)", render: (r) => <span className="cell-strong">{r.event.name}</span> },
            { key: "time", title: "时间", width: rows?.some((r) => r.timeText.length > 5) ? "152px" : "72px", render: (r) => r.timeText },
            { key: "kind", title: "类型", width: "72px", render: (r) => kindLabels[r.event.kind] },
            { key: "format", title: "赛制", width: "minmax(104px, 1fr)", render: (r) => r.formatText },
            { key: "entries", title: "报名", width: "72px", align: "end", render: (r) => r.entries },
            { key: "heats", title: "组", width: "56px", align: "end", render: (r) => r.heats },
            { key: "state", title: "进度", width: "128px", render: progress },
            { key: "actions", title: "", width: "88px", align: "end",
              render: (r) => (
                <span className="cell-actions" onClick={(e) => e.stopPropagation()}>
                  <StIconButton size="small" label="编辑" onClick={() => setEditing({ row: r })}>
                    <IconEdit size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton size="small" label="删除" onClick={() => askDelete(r)}>
                    <IconDelete size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                </span>
              ) },
          ]} />
      </div>
      <EventDialog initialGroupId={group === "all" ? "" : group} meetId={meetId} open={editing !== null} row={editing?.row} onClose={() => setEditing(null)} />
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.row.event.name ?? ""}」？`} onClose={() => setDeleting(null)} onDelete={() => EventsController.remove(deleting!.row.event.id)}>
        将删除 {deleting?.counts.heats} 组与 {deleting?.counts.results} 条成绩，无法撤销。
      </DeleteDialog>
    </StPage>
  );
}
