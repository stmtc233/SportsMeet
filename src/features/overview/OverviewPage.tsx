import { IconCheck, IconPrint, StButton, StEmpty, StListItem, StPage, StProgress, StSection, StTag, StToolbar, iconStroke, type Tone } from "../../design";
import type { OverviewPhase, OverviewRound, OverviewStep } from "../../mygo";
import { useNav, type MeetTab } from "../../app/nav";
import { remember } from "../../lib/viewState";
import { nodeKey, rosterDetailKey, rosterNodeKey } from "../entries/tree";
import { useMeet } from "../meets/data";
import { reportKindKey, reportKinds, type ReportKind } from "../reports/kinds";
import { useOverview } from "./data";
import { StructureMap } from "./StructureMap";

type StepView = { title: string; tab: MeetTab; value: string; text: string; progress: boolean; tags: { tone: Tone; text: string }[] };

const tag = (n: number, tone: Tone, text: string) => (n > 0 ? [{ tone, text: `${text} ${n}` }] : []);

/** How a step reads: its numbers, what they count, and what is worth a look. */
function stepView(s: OverviewStep): StepView {
  const ratio = `${s.done} / ${s.total}`;
  switch (s.key) {
    case "events":
      return { title: "项目", tab: "events", value: String(s.total), text: "个项目", progress: false, tags: tag(s.extra, "neutral", "全能") };
    case "units":
      return { title: "参赛单位", tab: "entries", value: String(s.total), text: "个单位", progress: false, tags: tag(s.warnings, "warning", "未分组") };
    case "entries":
      return { title: "报名", tab: "entries", value: ratio, text: "个单位已报名", progress: true, tags: tag(s.warnings, "warning", "超限") };
    case "arrange":
      return { title: "编排", tab: "events", value: ratio, text: "个项目已编排", progress: true, tags: [] };
    case "schedule":
      return { title: "日程", tab: "schedule", value: ratio, text: "个轮次已排时间", progress: true, tags: tag(s.warnings, "warning", "冲突") };
    case "results":
      return { title: "成绩", tab: "events", value: ratio, text: "个轮次已完成", progress: true, tags: tag(s.extra, "info", "进行中") };
    case "advance":
      return { title: "晋级", tab: "events", value: ratio, text: "个轮次已晋级", progress: true, tags: [...tag(s.extra, "info", "待晋级"), ...tag(s.warnings, "warning", "待裁定")] };
    default:
      return s.state === "off"
        ? { title: "团体总分", tab: "team-scoring", value: "未启用", text: "", progress: false, tags: [] }
        : { title: "团体总分", tab: "team-scoring", value: ratio, text: "个项目已计分", progress: true, tags: [] };
  }
}

/** Each phase's title and what it prints. */
const phases: Record<string, { title: string; prints: ReportKind[] }> = {
  before: { title: "赛前", prints: ["entries", "startlist"] },
  during: { title: "比赛", prints: ["checkin", "record"] },
  after: { title: "赛后", prints: ["results", "team_scores"] },
};

const tones: Record<string, Tone> = { done: "success", doing: "info", todo: "neutral", off: "neutral" };

/** 总览: the workflow phase by phase, the rounds still to be held, and the meet at a glance. */
export function OverviewPage({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data } = useOverview(meetId);
  const meet = useMeet(meetId).data;

  const openRound = (r: OverviewRound) => go({ page: "event", meetId, eventId: r.eventId, tab: r.heats > 0 ? "results" : "arrange" });
  const openStep = (s: OverviewStep) => {
    // Units of no group open selected on 报名, the tree in view.
    if (s.key === "units" && s.warnings > 0) {
      remember(rosterNodeKey(meetId), nodeKey({ group: "", unit: "" }));
      remember(rosterDetailKey(meetId), false);
    }
    go({ page: "meet", meetId, tab: stepView(s).tab });
  };
  const openReport = (kind: ReportKind) => {
    remember(reportKindKey, kind, true);
    go({ page: "meet", meetId, tab: "reports" });
  };
  const limits = meet ? [
    meet.entryLimits.athleteEvents > 0 && `每人限报 ${meet.entryLimits.athleteEvents} 项`,
    meet.entryLimits.unitAthletes > 0 && `每项限报 ${meet.entryLimits.unitAthletes} 人`,
    meet.entryLimits.unitTeams > 0 && `每项限报 ${meet.entryLimits.unitTeams} 队`,
  ].filter(Boolean).join("，") : "";
  // Steps are numbered on through the phases.
  const starts = (data?.phases ?? []).reduce<number[]>((all, ph, i, list) => [...all, i === 0 ? 0 : all[i - 1]! + list[i - 1]!.steps.length], []);

  return (
    <StPage scrollKey={`overview:${meetId}`} toolbar={<StToolbar title="总览" subtitle={meet?.datesText ?? ""} />}>
      {data && (
        <div className="overview view-enter">
          <div className="overview__phases">
            {data.phases.map((ph, i) => <Phase key={ph.key} phase={ph} start={starts[i]!} current={data.current} onStep={openStep} onPrint={openReport} />)}
          </div>
          <StSection title="赛事结构">
            <StructureMap meetId={meetId} meetName={meet?.name ?? ""} athletes={data.athletes} branches={data.structure ?? []} />
          </StSection>
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

/** A phase: its title with what it prints, then its steps. */
function Phase({ phase, start, current, onStep, onPrint }: {
  phase: OverviewPhase;
  start: number;
  current: string;
  onStep: (s: OverviewStep) => void;
  onPrint: (kind: ReportKind) => void;
}) {
  const p = phases[phase.key];
  // 团体总分 prints once it is enabled.
  const teamOff = phase.steps.some((s) => s.key === "team" && s.state === "off");
  const prints = (p?.prints ?? []).filter((k) => !(k === "team_scores" && teamOff));
  return (
    <section className="overview-phase">
      <div className="overview-phase__head">
        <h2 className="overview-phase__title">{p?.title}</h2>
        {prints.map((k) => (
          <StButton key={k} size="small" variant="ghost" icon={<IconPrint size={16} strokeWidth={iconStroke} />} onClick={() => onPrint(k)}>
            {reportKinds.find((r) => r.value === k)?.label}
          </StButton>
        ))}
      </div>
      <ol className="overview__steps">
        {phase.steps.map((s, i) => <Step key={s.key} index={start + i + 1} step={s} current={s.key === current} onOpen={() => onStep(s)} />)}
      </ol>
    </section>
  );
}

function Step({ index, step, current, onOpen }: { index: number; step: OverviewStep; current: boolean; onOpen: () => void }) {
  const v = stepView(step);
  const tone = tones[step.state] ?? "neutral";
  return (
    <li>
      <button type="button" className="overview-step" data-state={step.state} data-current={current || undefined} aria-current={current ? "step" : undefined} onClick={onOpen}>
        <span className="overview-step__head">
          <span className={`overview-step__index st-tone-${tone}`} aria-hidden>
            {step.state === "done" ? <IconCheck size={14} strokeWidth={iconStroke + 0.75} /> : index}
          </span>
          {v.title}
          {current && <span className="overview-step__next" aria-hidden>下一步</span>}
        </span>
        <span className="overview-step__value">{v.value}</span>
        {v.text && <span className="overview-step__text">{v.text}</span>}
        {v.progress && <StProgress value={step.done} max={step.total} tone={tone} label={`${v.title} ${v.value}`} />}
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
