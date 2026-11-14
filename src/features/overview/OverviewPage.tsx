import { IconCheck, StButton, StEmpty, StListItem, StPage, StProgress, StSection, StTag, StToolbar, iconStroke, type Tone } from "../../design";
import type { OverviewRound, OverviewStep } from "../../mygo";
import { useNav, type MeetTab } from "../../app/nav";
import { useMeet } from "../meets/data";
import { useOverview } from "./data";

type StepView = { title: string; tab: MeetTab; value: string; text: string; tags: { tone: Tone; text: string }[] };

/** How a workflow stage reads: its numbers, what they count, and what is worth a look. */
function stepView(s: OverviewStep): StepView {
  const ratio = `${s.done} / ${s.total}`;
  switch (s.key) {
    case "events":
      return { title: "项目", tab: "events", value: String(s.total), text: "个项目", tags: s.extra > 0 ? [{ tone: "neutral", text: `全能 ${s.extra}` }] : [] };
    case "entries":
      return { title: "报名", tab: "entries", value: ratio, text: "个单位已报名", tags: s.warnings > 0 ? [{ tone: "warning", text: `超限 ${s.warnings}` }] : [] };
    case "arrange":
      return { title: "编排", tab: "events", value: ratio, text: "个项目已编排", tags: [] };
    case "schedule":
      return { title: "日程", tab: "schedule", value: ratio, text: "个轮次已排时间", tags: s.warnings > 0 ? [{ tone: "warning", text: `冲突 ${s.warnings}` }] : [] };
    default:
      return { title: "成绩", tab: "events", value: ratio, text: "个项目已完成", tags: s.extra > 0 ? [{ tone: "info", text: `进行中 ${s.extra}` }] : [] };
  }
}

const tones: Record<string, Tone> = { done: "success", doing: "info", todo: "neutral" };

/** 总览: the workflow with each stage's state, the rounds still to be held, and the meet at a glance. */
export function OverviewPage({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data } = useOverview(meetId);
  const meet = useMeet(meetId).data;

  const openRound = (r: OverviewRound) => go({ page: "event", meetId, eventId: r.eventId, tab: r.heats > 0 ? "results" : "arrange" });
  const limits = meet ? [
    meet.entryLimits.athleteEvents > 0 && `每人限报 ${meet.entryLimits.athleteEvents} 项`,
    meet.entryLimits.unitAthletes > 0 && `每项限报 ${meet.entryLimits.unitAthletes} 人`,
    meet.entryLimits.unitTeams > 0 && `每项限报 ${meet.entryLimits.unitTeams} 队`,
  ].filter(Boolean).join("，") : "";

  return (
    <StPage scrollKey={`overview:${meetId}`} toolbar={<StToolbar title="总览" subtitle={meet?.datesText ?? ""} />}>
      {data && (
        <div className="overview view-enter">
          <ol className="overview__steps">
            {data.steps.map((s, i) => <Step key={s.key} index={i + 1} step={s} onOpen={(tab) => go({ page: "meet", meetId, tab })} />)}
          </ol>
          <div className="overview__columns">
            <StSection title="待比赛" padded={false}
              actions={data.nextTotal > data.next.length && <StButton size="small" variant="ghost" onClick={() => go({ page: "meet", meetId, tab: "schedule" })}>全部 {data.nextTotal} 个</StButton>}>
              {data.next.length === 0 ? <StEmpty>没有待比赛的轮次</StEmpty> : data.next.map((r) => (
                <StListItem key={`${r.eventId}:${r.roundLabel}`} title={`${r.eventName} ${r.roundLabel}`}
                  subtitle={r.heats > 0 ? `${r.startText} · ${r.heats} 组` : `${r.startText} · 未编排`}
                  trailing={r.started ? <StTag tone="info">进行中</StTag> : undefined} onClick={() => openRound(r)} />
              ))}
            </StSection>
            <StSection title="赛事">
              <dl className="overview__facts">
                <Fact label="日期" value={meet?.datesText} />
                <Fact label="地点" value={meet?.location} />
                <Fact label="参赛单位" value={String(data.units)} />
                <Fact label="运动员" value={String(data.athletes)} />
                {data.groups > 0 && <Fact label="组别" value={String(data.groups)} />}
                <Fact label="报名限制" value={limits || "不限"} />
              </dl>
            </StSection>
          </div>
        </div>
      )}
    </StPage>
  );
}

function Step({ index, step, onOpen }: { index: number; step: OverviewStep; onOpen: (tab: MeetTab) => void }) {
  const v = stepView(step);
  const tone = tones[step.state] ?? "neutral";
  return (
    <li>
      <button type="button" className="overview-step" data-state={step.state} onClick={() => onOpen(v.tab)}>
        <span className="overview-step__head">
          <span className={`overview-step__index st-tone-${tone}`} aria-hidden>
            {step.state === "done" ? <IconCheck size={14} strokeWidth={iconStroke + 0.75} /> : index}
          </span>
          {v.title}
        </span>
        <span className="overview-step__value">{v.value}</span>
        <span className="overview-step__text">{v.text}</span>
        {step.key !== "events" && <StProgress value={step.done} max={step.total} tone={tone} label={`${v.title} ${v.value}`} />}
        {v.tags.length > 0 && <span className="overview-step__tags">{v.tags.map((t) => <StTag key={t.text} tone={t.tone}>{t.text}</StTag>)}</span>}
      </button>
    </li>
  );
}

function Fact({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </>
  );
}
