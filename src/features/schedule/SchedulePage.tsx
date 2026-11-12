import { useState } from "react";
import { useNav } from "../../app/nav";
import { IconChevronDown, IconChevronRight, IconEdit, IconTime, StButton, StEmpty, StIconButton, StMuted, StPage, StSection, StTable, StTag, StToolbar, iconStroke } from "../../design";
import type { ScheduleHeatRef, Timetable, TimetableHeat, TimetableRound } from "../../mygo";
import { ConflictList } from "../arrange/ConflictList";
import { ConflictTag } from "../arrange/HeatCard";
import { HeatDialog } from "../arrange/HeatDialog";
import { RoundStartDialog } from "../arrange/RoundStartDialog";
import { ScheduleDialog } from "../arrange/ScheduleDialog";
import { HeatTime } from "../event/HeatTime";
import { useMeet } from "../meets/data";
import { ScheduleController, useTimetable } from "./controller";
import { scheduleOptions } from "./options";

/**
 * The meet's competition schedule: rounds and heats by day and effective
 * time, those with no time apart (folded), and the athletes' time
 * conflicts. Times are edited with the arrange page's dialogs and Go's
 * entry points.
 */
export function SchedulePage({ meetId }: { meetId: string }) {
  const data = useTimetable(meetId).data ?? undefined;
  const meet = useMeet(meetId).data;
  const [chaining, setChaining] = useState(false);
  const [unfolded, setUnfolded] = useState(false);
  const [starting, setStarting] = useState<TimetableRound | null>(null);
  const [editing, setEditing] = useState<{ round: TimetableRound; heat: TimetableHeat } | null>(null);
  const defaultDay = data?.days[0]?.day || meet?.startDate || "";
  const empty = data !== undefined && data.days.length === 0 && data.unscheduled.length === 0;
  const edit = (round: TimetableRound, heat: TimetableHeat) => setEditing({ round, heat });
  return (
    <StPage
      toolbar={
        <StToolbar title="日程">
          <StButton icon={<IconTime size={16} strokeWidth={iconStroke} />} disabled={!data || empty} onClick={() => setChaining(true)}>连排…</StButton>
        </StToolbar>
      }>
      <div className="narrow timetable">
        {empty && <StEmpty>还没有项目</StEmpty>}
        {data && !empty && data.days.length === 0 && <StEmpty>还没有安排时间</StEmpty>}
        {data && data.conflicts.length > 0 && <Conflicts meetId={meetId} data={data} />}
        {data?.days.map((d) => <Day key={d.day} title={d.day} rounds={d.rounds} meetId={meetId} onStart={setStarting} onEdit={edit} />)}
        {data && data.unscheduled.length > 0 && (
          <Day title="未安排时间" count={data.unscheduled.length} folded={!unfolded} onFold={() => setUnfolded(!unfolded)}
            rounds={data.unscheduled} meetId={meetId} onStart={setStarting} onEdit={edit} />
        )}
      </div>
      {chaining && data && <ScheduleDialog multi options={scheduleOptions(data)} day={defaultDay} version={data} onClose={() => setChaining(false)} />}
      {starting && (
        <RoundStartDialog eventId={starting.eventId} day={starting.day || defaultDay} onClose={() => setStarting(null)}
          round={{ id: starting.roundId, label: `${starting.eventName} ${starting.roundLabel}`, scheduledAt: starting.scheduledAt }} />
      )}
      <HeatDialog heat={editing?.heat ?? null} eventId={editing?.round.eventId ?? ""} day={editing?.round.day || defaultDay}
        round={{ startAt: editing?.round.scheduledAt ?? "", startText: editing?.round.scheduledText ?? "" }}
        onClose={() => setEditing(null)}
        onSave={(patch) => {
          if (!editing) return;
          void ScheduleController.editHeat({ eventId: editing.round.eventId, heatId: editing.heat.id, base: editing.heat.base,
            name: patch.name, scheduledAt: patch.scheduledAt, durationMinutes: patch.durationMinutes });
        }} />
    </StPage>
  );
}

function Conflicts({ meetId, data }: { meetId: string; data: Timetable }) {
  const { go } = useNav();
  const open = (ref: ScheduleHeatRef) => go({ page: "event", meetId, eventId: ref.eventId, tab: "arrange" });
  return (
    <StSection padded={false} title={<>时间冲突 <StTag tone="danger">{data.conflicts.length}</StTag></>}
      actions={data.minRestMinutes > 0 ? <StMuted>最短休息 {data.minRestMinutes} 分钟</StMuted> : undefined}>
      <div className="timetable__conflicts">
        <ConflictList conflicts={data.conflicts} onOpen={open} />
      </div>
    </StSection>
  );
}

function Day({ title, count, folded = false, onFold, rounds, meetId, onStart, onEdit }: {
  title: string;
  /** With onFold: the section folds, its title showing the count. */
  count?: number;
  folded?: boolean;
  onFold?: () => void;
  rounds: TimetableRound[];
  meetId: string;
  onStart: (round: TimetableRound) => void;
  onEdit: (round: TimetableRound, heat: TimetableHeat) => void;
}) {
  const { go } = useNav();
  return (
    <section className="timetable__day" aria-label={title}>
      <div className="timetable__day-head">
        <h2 className="timetable__day-title">{title}</h2>
        {count !== undefined && <StTag>{count}</StTag>}
        {onFold && (
          <StButton size="small" variant="ghost" aria-expanded={!folded} reserveWidthFor={["展开", "收起"]}
            icon={folded ? <IconChevronRight size={16} strokeWidth={iconStroke} /> : <IconChevronDown size={16} strokeWidth={iconStroke} />} onClick={onFold}>
            {folded ? "展开" : "收起"}
          </StButton>
        )}
      </div>
      {!folded && rounds.map((r) => (
        <StSection key={r.roundId} padded={false}
          title={
            <span className="timetable__round">
              <span className="timetable__time">{r.startText}</span>
              <span>{r.eventName} {r.roundLabel}</span>
              <StMuted>{r.peopleEntered ? `报名 ${r.people} 人` : `${r.heatCount} 组 · ${r.people} 人`}</StMuted>
            </span>
          }
          actions={
            <>
              <StButton size="small" variant="ghost" icon={<IconTime size={16} strokeWidth={iconStroke} />} title="轮次开始时间" onClick={() => onStart(r)}>
                {r.scheduledText || "开始时间"}
              </StButton>
              <StButton size="small" variant="ghost" onClick={() => go({ page: "event", meetId, eventId: r.eventId, tab: "arrange" })}>编排</StButton>
            </>
          }>
          {r.heats.length > 0 && (
            <StTable<TimetableHeat> dense rows={r.heats} rowKey={(h) => h.id} columns={[
              { key: "time", title: "时间", width: "minmax(132px, 1.5fr)", render: (h) => <HeatTime text={h.scheduledText} inherited={h.scheduleInherited} /> },
              { key: "heat", title: "组", width: "minmax(72px, 1fr)", render: (h) => <span title={h.label}>{h.label}</span> },
              { key: "duration", title: "用时", render: (h) => h.durationText },
              { key: "end", title: "结束", width: "minmax(64px, 1fr)", render: (h) => h.endText },
              { key: "people", title: "人数", align: "end", render: (h) => h.people },
              { key: "state", title: "状态", width: "minmax(96px, 1fr)", render: (h) => (
                <span className="timetable__state">
                  {h.keepReason && <StTag>{h.keepReason}</StTag>}
                  {h.conflicts.length > 0 && <ConflictTag conflicts={h.conflicts} />}
                </span>
              ) },
              { key: "edit", title: "", width: "48px", render: (h) => (
                <StIconButton size="small" label="编辑组" onClick={() => onEdit(r, h)}><IconEdit size={16} strokeWidth={iconStroke} /></StIconButton>
              ) },
            ]} />
          )}
        </StSection>
      ))}
    </section>
  );
}
