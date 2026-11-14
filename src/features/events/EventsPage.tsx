import { useState } from "react";
import { lastEventTab, useNav } from "../../app/nav";
import { IconAdd, IconDelete, IconEdit, StButton, StEmpty, StIconButton, StPage, StSegmented, StTable, StTag, StToolbar, iconStroke } from "../../design";
import { kindLabels } from "../../lib/labels";
import { useViewState } from "../../lib/viewState";
import type { CombinedDetail, CombinedRowSummary, EventRow, MeetCounts } from "../../mygo";
import { CombinedDialog } from "../combined/CombinedDialog";
import { CombinedController, useCombinedList } from "../combined/controller";
import { GroupsController, useGroups } from "../groups/controller";
import { GroupFilter, GroupMenu, inGroup, useGroupFilter } from "../groups/GroupFields";
import { DeleteDialog } from "../meets/DeleteDialog";
import { EventsController, useEvents } from "./controller";
import { EventDialog } from "./EventDialog";

type Kind = "athlete" | "team" | "combined";
type Row = { key: string; kind: Kind; name: string; groupId: string } & ({ event: EventRow; combined?: undefined } | { combined: CombinedRowSummary; event?: undefined });

const kindOptions: { value: Kind | "all"; label: string }[] = [
  { value: "all", label: "全部" }, { value: "athlete", label: "个人" }, { value: "team", label: "队伍" }, { value: "combined", label: "全能" },
];

function progress(row: EventRow) {
  if (row.heats === 0) return <StTag>未编排</StTag>;
  if (row.results === 0) return <StTag tone="info">已编排</StTag>;
  if (row.results < row.placed) return <StTag tone="warning">录入中 {row.results}/{row.placed}</StTag>;
  return <StTag tone="success">已完成</StTag>;
}

/** The meet's events and combined events in one list; components live under their combined event. */
function rowsOf(events: EventRow[] | undefined, combineds: CombinedRowSummary[] | undefined): Row[] | undefined {
  if (!events || !combineds) return undefined;
  return [
    ...events.filter((r) => !r.event.combinedId).map((r): Row => ({
      key: `e:${r.event.id}`, kind: r.event.kind === "relay" || r.event.participantMode === "team" ? "team" : "athlete", name: r.event.name, groupId: r.event.groupId ?? "", event: r,
    })),
    ...combineds.map((r): Row => ({ key: `c:${r.combined.id}`, kind: "combined", name: r.combined.name, groupId: r.combined.groupId ?? "", combined: r })),
  ];
}

export function EventsPage({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data: events } = useEvents(meetId);
  const { data: combineds } = useCombinedList(meetId);
  const { data: groups } = useGroups(meetId);
  const [group, setGroup] = useGroupFilter(meetId, "events");
  const [savedKind, setKind] = useViewState<Kind | "all">(`events:kind:${meetId}`, "all");
  const all = rowsOf(events, combineds);
  const kinds = new Set(all?.map((r) => r.kind));
  // One kind of event has nothing to filter by.
  const kind = kinds.size > 1 && kinds.has(savedKind as Kind) ? savedKind : "all";
  const rows = all?.filter((r) => (kind === "all" || r.kind === kind) && inGroup(group, r.groupId));
  const [selected, setSelected] = useState(new Set<string>());
  const picked = (rows ?? []).filter((r) => selected.has(r.key));
  const [editing, setEditing] = useState<{ row?: EventRow } | null>(null);
  const [editingCombined, setEditingCombined] = useState<{ detail?: CombinedDetail } | null>(null);
  const [deleting, setDeleting] = useState<{ row: EventRow; counts: MeetCounts } | null>(null);
  const [deletingCombined, setDeletingCombined] = useState<CombinedDetail | null>(null);

  const open = (r: Row) => go(r.event ? { page: "event", meetId, eventId: r.event.event.id, tab: lastEventTab(r.event.event.id) } : { page: "combined", meetId, combinedId: r.combined.combined.id });
  const edit = async (r: Row) => {
    if (r.event) return setEditing({ row: r.event });
    const detail = await CombinedController.get(r.combined.combined.id);
    if (detail) setEditingCombined({ detail });
  };
  const askDelete = async (r: Row) => {
    if (r.event) {
      const counts = await EventsController.count(r.event.event.id);
      if (counts) setDeleting({ row: r.event, counts });
      return;
    }
    const detail = await CombinedController.get(r.combined.combined.id);
    if (detail) setDeletingCombined(detail);
  };
  const initialGroupId = group === "all" ? "" : group;

  return (
    <StPage flush
      toolbar={
        <StToolbar title="项目" subtitle={rows ? `${rows.length} 项` : ""}>
          <GroupMenu meetId={meetId} count={picked.length}
            onPick={(id) => GroupsController.move(meetId, id, picked.flatMap((r) => (r.event ? [r.event.event.id] : [])), picked.flatMap((r) => (r.combined ? [r.combined.combined.id] : [])))} />
          <StButton onClick={() => setEditingCombined({})}>添加全能</StButton>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加项目</StButton>
        </StToolbar>
      }>
      <div className="filters">
        {kinds.size > 1 && <StSegmented aria-label="类型" value={kind} onChange={setKind} options={kindOptions.filter((o) => o.value === "all" || kinds.has(o.value))} />}
        {!!groups?.length && <GroupFilter meetId={meetId} value={group} onChange={setGroup} />}
      </div>
      <div className="table-card">
        <StTable<Row> fill rows={rows ?? []} selected={selected} onSelect={setSelected} loading={!rows} scrollKey={`events:${meetId}`} rowKey={(r) => r.key} rowLabel={(r) => r.name} actionColumn="name" onRowClick={open}
          empty={<StEmpty action={<StButton variant="primary" onClick={() => setEditing({})}>添加项目</StButton>}>{all?.length ? "没有符合条件的项目" : "还没有项目"}</StEmpty>}
          columns={[
            { key: "name", title: "项目", width: "minmax(160px, 2fr)", render: (r) => <span className="cell-strong">{r.name}</span> },
            { key: "time", title: "时间", width: all?.some((r) => (r.event?.timeText.length ?? 0) > 5) ? "152px" : "72px", render: (r) => r.event?.timeText },
            { key: "kind", title: "类型", width: "72px", render: (r) => (r.event ? kindLabels[r.event.event.kind] : "全能") },
            { key: "format", title: "赛制", width: "minmax(104px, 1fr)", render: (r) => (r.event ? r.event.formatText : `${r.combined.components} 个分项`) },
            { key: "entries", title: "报名", width: "72px", align: "end", render: (r) => (r.event ?? r.combined).entries },
            { key: "heats", title: "组", width: "56px", align: "end", render: (r) => r.event?.heats },
            { key: "state", title: "进度", width: "128px", render: (r) => r.event && progress(r.event) },
            { key: "actions", title: "", width: "88px", align: "end",
              render: (r) => (
                <span className="cell-actions" onClick={(e) => e.stopPropagation()}>
                  <StIconButton size="small" label="编辑" onClick={() => void edit(r)}>
                    <IconEdit size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton size="small" label="删除" onClick={() => void askDelete(r)}>
                    <IconDelete size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                </span>
              ) },
          ]} />
      </div>
      <EventDialog initialGroupId={initialGroupId} meetId={meetId} open={editing !== null} row={editing?.row} onClose={() => setEditing(null)} />
      {editingCombined && <CombinedDialog initialGroupId={initialGroupId} meetId={meetId} detail={editingCombined.detail} onClose={() => setEditingCombined(null)} />}
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.row.event.name ?? ""}」？`} onClose={() => setDeleting(null)} onDelete={() => EventsController.remove(deleting!.row.event.id)}>
        将删除 {deleting?.counts.heats} 组与 {deleting?.counts.results} 条成绩，无法撤销。
      </DeleteDialog>
      <DeleteDialog open={deletingCombined !== null} title={`删除「${deletingCombined?.combined.name ?? ""}」？`} onClose={() => setDeletingCombined(null)} onDelete={() => CombinedController.remove(deletingCombined!.combined.id)}>
        将删除 {deletingCombined?.components.length ?? 0} 个分项、{deletingCombined?.heats ?? 0} 组与 {deletingCombined?.results ?? 0} 条成绩，无法撤销。
      </DeleteDialog>
    </StPage>
  );
}
