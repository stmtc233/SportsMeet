import { useState } from "react";
import { useNav } from "../../app/nav";
import { IconAuto, StButton, StEmpty, StPage, StSegmented, StTable, StToolbar, iconStroke } from "../../design";
import { remember, useViewState } from "../../lib/viewState";
import type { RoundRow } from "../../mygo";
import { useGroups } from "../groups/controller";
import { GroupFilter, inGroup, useGroupFilter } from "../groups/GroupFields";
import { BatchArrangeDialog } from "./BatchArrangeDialog";
import { useMeetRounds } from "./controller";
import { ArrangeState } from "./states";

type Filter = "ready" | "all";

/** Opens a round on its event's page: the round chosen there, at the tab given. */
export function useOpenRound(meetId: string) {
  const { go } = useNav();
  return (r: RoundRow, tab: "arrange" | "results") => {
    remember(`event:round:${r.eventId}`, r.roundId);
    go({ page: "event", meetId, eventId: r.eventId, tab });
  };
}

/**
 * 编排: every round of the meet and where it stands. Rounds picked here are
 * arranged automatically in one go; a row opens its event's 编排.
 */
export function ArrangePage({ meetId }: { meetId: string }) {
  const { data } = useMeetRounds(meetId);
  const { data: groups } = useGroups(meetId);
  const [group, setGroup] = useGroupFilter(meetId, "arrange");
  const [savedFilter, setFilter] = useViewState<Filter>(`arrange:filter:${meetId}`, "ready");
  const [selected, setSelected] = useState(new Set<string>());
  const [batch, setBatch] = useState<RoundRow[] | null>(null);
  const open = useOpenRound(meetId);
  const all = data?.rounds.filter((r) => inGroup(group, r.groupId));
  const ready = all?.filter((r) => r.canArrange) ?? [];
  // Nothing left to arrange shows them all.
  const filter = ready.length === 0 ? "all" : savedFilter;
  const rows = filter === "ready" ? ready : all;
  const picked = (rows ?? []).filter((r) => selected.has(r.roundId) && r.canArrange);
  const multi = (data?.rounds ?? []).some((r) => !r.first);

  return (
    <StPage flush
      toolbar={
        <StToolbar title="编排" subtitle={data ? (ready.length > 0 ? `${ready.length} 个轮次可编排` : "") : ""}>
          <StButton variant="primary" icon={<IconAuto size={16} strokeWidth={iconStroke} />} disabled={picked.length === 0}
            reserveWidthFor={["自动编排 99 个"]} onClick={() => setBatch(picked)}>
            {picked.length > 0 ? `自动编排 ${picked.length} 个` : "自动编排"}
          </StButton>
        </StToolbar>
      }>
      <div className="filters">
        <StSegmented aria-label="轮次" value={filter} onChange={setFilter}
          options={[{ value: "ready", label: `可编排 ${ready.length}` }, { value: "all", label: "全部" }]} />
        {!!groups?.length && <GroupFilter meetId={meetId} value={group} onChange={setGroup} />}
      </div>
      <div className="table-card">
        <StTable<RoundRow> fill rows={rows ?? []} loading={!data} scrollKey={`arrange:${meetId}`} rowKey={(r) => r.roundId} rowLabel={(r) => r.text}
          actionColumn="name" selected={selected} onSelect={setSelected} onRowClick={(r) => open(r, "arrange")}
          empty={<StEmpty>{data?.rounds.length ? "没有符合条件的轮次" : "还没有项目"}</StEmpty>}
          columns={[
            { key: "name", title: "项目", width: "minmax(160px, 2fr)", render: (r) => <span className="cell-strong">{r.eventName}</span> },
            ...(multi ? [{ key: "round", title: "轮次", width: "88px", render: (r: RoundRow) => r.roundLabel }] : []),
            { key: "source", title: "来源", width: "minmax(96px, 1fr)", render: (r) => r.source },
            { key: "people", title: "人数", width: "72px", align: "end", render: (r) => (r.heats > 0 ? r.people : r.canArrange ? r.unplaced : "") },
            { key: "heats", title: "组", width: "56px", align: "end", render: (r) => r.heats || "" },
            { key: "time", title: "时间", width: "minmax(72px, 1fr)", render: (r) => r.timeText },
            { key: "state", title: "状态", width: "minmax(132px, 1.2fr)", render: (r) => <ArrangeState row={r} /> },
          ]} />
      </div>
      {batch && <BatchArrangeDialog meetId={meetId} rounds={batch} onClose={() => setBatch(null)} onDone={() => { setBatch(null); setSelected(new Set()); }} />}
    </StPage>
  );
}
