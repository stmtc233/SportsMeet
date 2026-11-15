import type { CSSProperties } from "react";
import { StButton, StEmpty, StSection, StSegmented, StSelect, StTag, type Tone } from "../../design";
import type { OverviewTimeline, TimelineDay, TimelineRound } from "../../mygo";
import { useNav } from "../../app/nav";
import { remember, useViewState } from "../../lib/viewState";

const laneNames: Record<string, string> = { track: "径赛", field: "田赛", other: "其他" };

/** A round's state as the 项目 page and 赛事结构 colour it. */
const states: Record<string, { tone: Tone; text: string }> = {
  todo: { tone: "neutral", text: "未编排" },
  arranged: { tone: "info", text: "已编排" },
  doing: { tone: "warning", text: "进行中" },
  done: { tone: "success", text: "已完成" },
};

const stateOf = (r: TimelineRound) => states[r.state] ?? states.todo!;

/** What a round's block says when hovered or read aloud. */
function roundLabel(r: TimelineRound) {
  const parts = [r.text, r.timeText, r.heats > 0 ? `${r.heats} 组` : "未编排"];
  if (r.people > 0) parts.push(r.peopleEntered ? `报名 ${r.people} 人` : `${r.people} 人`);
  if (r.heats > 0) parts.push(stateOf(r).text);
  if (r.late) parts.push("已过开始时间");
  if (r.next) parts.push("下一场");
  return parts.join(" · ");
}

/** 赛程: a day's rounds hour by hour, a column per lane, with now marked. */
export function Timeline({ meetId, timeline }: { meetId: string; timeline: OverviewTimeline }) {
  const { go } = useNav();
  const [saved, setDay] = useViewState(`overview:day:${meetId}`, "");
  const days = timeline.days;
  const day = days.find((d) => d.day === saved) ?? days.find((d) => d.day === timeline.day) ?? days[0];
  const open = (r: TimelineRound) => {
    remember(`event:round:${r.eventId}`, r.roundId);
    go({ page: "event", meetId, eventId: r.eventId, tab: r.heats > 0 ? "results" : "arrange" });
  };
  const schedule = () => go({ page: "meet", meetId, tab: "schedule" });
  const options = days.map((d) => ({ value: d.day, label: d.text }));

  return (
    <StSection title="赛程" actions={<>
      {days.length > 1 && (days.length <= 5
        ? <StSegmented size="small" aria-label="日子" value={day!.day} options={options} onChange={setDay} />
        : <StSelect size="small" aria-label="日子" value={day!.day} options={options} onChange={setDay} />)}
      <StButton size="small" variant="ghost" onClick={schedule}>全部日程</StButton>
    </>}>
      {!day ? <StEmpty>还没有安排时间</StEmpty> : <>
        <Table day={day} onOpen={open} />
      </>}
      {timeline.unscheduled > 0 && (
        <div className="timeline__more">
          <StButton size="small" variant="ghost" onClick={schedule}>{timeline.unscheduled} 个轮次未安排时间</StButton>
        </div>
      )}
    </StSection>
  );
}

type Vars = CSSProperties & Record<`--${string}`, number>;

/** The day as a table: an hour a row, a lane a column; narrow windows stack the lanes. */
function Table({ day, onOpen }: { day: TimelineDay; onOpen: (r: TimelineRound) => void }) {
  return (
    <div className="timeline" style={{ "--lanes": day.lanes.length } as Vars}>
      <div className="timeline__row timeline__head" aria-hidden>
        <span />
        {day.lanes.map((l) => <span key={l} className="timeline__lane-name">{laneNames[l]}</span>)}
      </div>
      {day.rows.map((row, i) => row.now ? (
        <div key={`now:${i}`} className="timeline__now"><span className="timeline__now-text">{row.text}</span></div>
      ) : (
        <div key={i} className="timeline__row" data-continued={row.text === "" || undefined}>
          <span className="timeline__hour">{row.text}</span>
          {row.cells.map((cell, c) => (
            <div key={day.lanes[c]} className="timeline__cell" data-empty={cell.length === 0 || undefined}>
              <span className="timeline__cell-lane">{laneNames[day.lanes[c]!]}</span>
              {cell.map((r) => <Round key={r.roundId} round={r} onOpen={onOpen} />)}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

/** A round: its start, name and, when worth a look, a tag; the bar colours its state. */
function Round({ round: r, onOpen }: { round: TimelineRound; onOpen: (r: TimelineRound) => void }) {
  return (
    <button type="button" className={`timeline__round st-tone-${stateOf(r).tone}`} data-state={r.state}
      title={roundLabel(r)} aria-label={roundLabel(r)} onClick={() => onOpen(r)}>
      <span className="timeline__time">{r.startText}</span>
      <span className="timeline__name">{r.text}</span>
      <Tag round={r} />
    </button>
  );
}

function Tag({ round: r }: { round: TimelineRound }) {
  if (r.late) return <StTag tone="danger">已过开始时间</StTag>;
  if (r.next) return <StTag tone="info">下一场</StTag>;
  if (r.state === "doing") return <StTag tone="warning">进行中</StTag>;
  return null;
}
