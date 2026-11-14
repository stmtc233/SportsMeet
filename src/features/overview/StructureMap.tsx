import type { ReactNode } from "react";
import { useNav } from "../../app/nav";
import { StTag, type Tone } from "../../design";
import { remember } from "../../lib/viewState";
import type { OverviewBranch, OverviewEvent, OverviewUnit } from "../../mygo";
import { nodeKey, rosterDetailKey, rosterNodeKey } from "../entries/tree";

/** Chips a branch shows before 「+N」 takes the rest to its list. */
const shownChips = 8;

const eventStates: Record<string, { tone: Tone; text: string }> = {
  todo: { tone: "neutral", text: "未编排" },
  arranged: { tone: "info", text: "已编排" },
  doing: { tone: "warning", text: "进行中" },
  done: { tone: "success", text: "已完成" },
};

/**
 * 赛事结构 on 总览: the meet → its groups → their units and events, as a
 * mind map. Every node opens where it is kept: a group or unit selected in
 * the 报名 tree, the events of a group on 项目, an event's own page.
 */
export function StructureMap({ meetId, meetName, athletes, branches }: { meetId: string; meetName: string; athletes: number; branches: OverviewBranch[] }) {
  const { go } = useNav();
  const openRoster = (group: string, unit = "") => {
    remember(rosterNodeKey(meetId), nodeKey({ group, unit }));
    remember(rosterDetailKey(meetId), unit !== "" || group !== "all");
    go({ page: "meet", meetId, tab: "entries" });
  };
  const openEvents = (group: string) => {
    remember(`events:group:${meetId}`, group);
    go({ page: "meet", meetId, tab: "events" });
  };
  const openEvent = (e: OverviewEvent) => {
    if (e.combined) return go({ page: "combined", meetId, combinedId: e.id });
    go({ page: "event", meetId, eventId: e.id, tab: e.state === "todo" ? "entries" : e.state === "arranged" ? "arrange" : "results" });
  };
  // A meet without groups hangs its units and events right off the meet.
  const flat = branches.length === 1 && branches[0]!.name === "";

  const leaves = (b: OverviewBranch) => {
    // The tree's node for the branch: its group, or 全部 when there are none.
    const group = flat ? "all" : b.groupId;
    const real = flat || b.groupId !== "";
    return (
      <>
        {(real || b.units.length > 0) && (
          <Leaf label="参赛单位" count={b.units.length} onOpen={() => openRoster(group)}
            chips={b.units.map((u) => <UnitChip key={u.id} unit={u} onOpen={() => openRoster(flat ? "all" : b.groupId, u.id)} />)} />
        )}
        {(real || b.events.length > 0) && (
          <Leaf label="项目" count={b.events.length} onOpen={() => openEvents(flat ? "all" : b.groupId)}
            chips={b.events.map((e) => <EventChip key={e.id} event={e} onOpen={() => openEvent(e)} />)} />
        )}
      </>
    );
  };

  return (
    <div className="mindmap">
      <button type="button" className="mindmap__node mindmap__node--root" onClick={() => openRoster("all")}>
        {meetName}<span className="mindmap__count">{athletes} 人</span>
      </button>
      <ul className="mindmap__list">
        {flat ? leaves(branches[0]!) : branches.map((b) => (
          <li key={b.groupId || "-"} className="mindmap__item">
            <button type="button" className="mindmap__node mindmap__node--group"
              onClick={() => (b.groupId === "" && b.units.length === 0 ? openEvents("") : openRoster(b.groupId))}>
              {b.name}<span className="mindmap__count">{b.athletes} 人</span>
            </button>
            <ul className="mindmap__list">{leaves(b)}</ul>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Leaf({ label, count, chips, onOpen }: { label: string; count: number; chips: ReactNode[]; onOpen: () => void }) {
  const more = chips.length - shownChips;
  return (
    <li className="mindmap__item">
      <button type="button" className="mindmap__node mindmap__node--leaf" onClick={onOpen}>
        {label}<span className="mindmap__count">{count}</span>
      </button>
      {chips.length > 0 && (
        <div className="mindmap__chips">
          {chips.slice(0, shownChips)}
          {more > 0 && <button type="button" className="mindmap__chip mindmap__chip--more" onClick={onOpen}>+{more}</button>}
        </div>
      )}
    </li>
  );
}

function UnitChip({ unit: u, onOpen }: { unit: OverviewUnit; onOpen: () => void }) {
  return (
    <button type="button" className="mindmap__chip" onClick={onOpen}>
      {u.name}<span className="mindmap__count">{u.athletes}</span>
      {!u.entered ? <StTag>未报名</StTag> : u.over > 0 ? <StTag tone="warning">超限 {u.over}</StTag> : null}
    </button>
  );
}

function EventChip({ event: e, onOpen }: { event: OverviewEvent; onOpen: () => void }) {
  const state = eventStates[e.state] ?? eventStates.todo!;
  return (
    <button type="button" className={`mindmap__chip st-tone-${state.tone}`} data-tone={state.tone} title={`${e.name} ${state.text}`} aria-label={`${e.name} ${state.text}`} onClick={onOpen}>
      {e.name}
    </button>
  );
}
