import { IconPrint, StButton, StEmpty, StMenu, StPage, StSegmented, StTable, StToolbar, iconStroke } from "../../design";
import { useViewState } from "../../lib/viewState";
import type { RoundRow } from "../../mygo";
import { useOpenReport } from "../reports/open";
import { useOpenRound } from "./ArrangePage";
import { useMeetRounds } from "./controller";
import { ResultsState } from "./states";

const unscheduled = "unscheduled";

/**
 * 成绩: the match day. One tab per day (today first, as Go picks it), its
 * rounds by time and where they stand; each prints its 检录单 and
 * 成绩记录表, and opens its event's 成绩.
 */
export function ResultsPage({ meetId }: { meetId: string }) {
  const { data } = useMeetRounds(meetId);
  const [savedDay, setDay] = useViewState(`results:day:${meetId}`, "");
  const open = useOpenRound(meetId);
  const print = useOpenReport(meetId);
  const tabs = [...(data?.days ?? []).map((d) => ({ value: d.day, label: d.text })), ...(data?.unscheduled.length ? [{ value: unscheduled, label: "未安排时间" }] : [])];
  // A remembered day that has gone opens Go's day.
  const day = tabs.some((t) => t.value === savedDay) ? savedDay : data?.day || tabs[0]?.value || "";
  const rows = day === unscheduled ? data?.unscheduled : data?.days.find((d) => d.day === day)?.rounds;
  const left = rows?.filter((r) => r.results !== "done" && r.results !== "advance").length ?? 0;

  return (
    <StPage flush
      toolbar={<StToolbar title="成绩" subtitle={rows?.length ? (left > 0 ? `还有 ${left} 个轮次` : "已全部录完") : ""} />}>
      {tabs.length > 1 && (
        <div className="filters">
          <StSegmented aria-label="日期" value={day} onChange={setDay} options={tabs} />
        </div>
      )}
      <div className="table-card">
        <StTable<RoundRow> fill rows={rows ?? []} loading={!data} scrollKey={`results:${meetId}:${day}`} rowKey={(r) => r.roundId} rowLabel={(r) => r.text}
          actionColumn="name" onRowClick={(r) => open(r, r.heats > 0 ? "results" : "arrange")}
          empty={<StEmpty>{data?.rounds.length ? "还没有安排时间的轮次" : "还没有项目"}</StEmpty>}
          columns={[
            { key: "time", title: "时间", width: "72px", render: (r) => r.startText },
            { key: "name", title: "项目", width: "minmax(120px, 2fr)", render: (r) => <span className="cell-strong">{r.text}</span> },
            { key: "heats", title: "组", width: "56px", align: "end", render: (r) => (r.heats > 0 ? `${r.heatsDone}/${r.heats}` : "") },
            { key: "people", title: "人数", width: "56px", align: "end", render: (r) => r.people || "" },
            { key: "state", title: "状态", width: "minmax(104px, 1fr)", render: (r) => <ResultsState row={r} /> },
            { key: "print", title: "", width: "88px", align: "end", render: (r) => r.heats > 0 && (
              <span className="cell-actions" onClick={(e) => e.stopPropagation()}>
                <StMenu items={[{ label: "检录单", onSelect: () => print("checkin", [r.roundId]) }, { label: "成绩记录表", onSelect: () => print("record", [r.roundId]) }]}
                  trigger={(p) => <StButton size="small" variant="ghost" icon={<IconPrint size={16} strokeWidth={iconStroke} />} aria-label={`打印${r.text}`} {...p}>打印</StButton>} />
              </span>
            ) },
          ]} />
      </div>
    </StPage>
  );
}
