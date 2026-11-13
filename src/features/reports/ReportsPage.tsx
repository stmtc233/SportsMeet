import { useRef, useState, type ReactNode } from "react";
import { IconPrint, StButton, StCheckbox, StEmpty, StNotice, StPage, StSegmented, StSelect, StToolbar, iconStroke } from "../../design";
import { useScrollMemory, useViewState } from "../../lib/viewState";
import type { Zoom } from "./paginate";
import { ReportPreview } from "./ReportPreview";
import { checkInRows, checkInSelection } from "./checkin";
import { ReportsController, useCheckInTargets, useReportHTML, useReportTargets, useReportTeamScores, useSheetHTML } from "./controller";

type Kind = "startlist" | "results" | "checkin" | "record" | "team_scores";
const kinds: Kind[] = ["startlist", "results", "checkin", "record", "team_scores"];

const zooms: { value: Zoom; label: string }[] = [
  { value: "fit", label: "适合宽度" }, { value: "0.5", label: "50%" }, { value: "0.75", label: "75%" }, { value: "1", label: "100%" }, { value: "1.5", label: "150%" },
];

/** 秩序册, 成绩册, 检录单 and 成绩记录表: rendered by Go, previewed on A4 sheets, printed from a window of their own. */
export function ReportsPage({ meetId }: { meetId: string }) {
  const [savedKind, setKind] = useViewState<Kind>("reports:kind", "startlist", true);
  const kind = kinds.includes(savedKind) ? savedKind : "startlist";
  const [busy, setBusy] = useState(false);
  const [savedZoom, setZoom] = useViewState<Zoom>("reports:zoom", "fit", true);
  const zoom = zooms.some((z) => z.value === savedZoom) ? savedZoom : "fit";
  const [pages, setPages] = useState(0);
  // 秩序册 and 成绩册: by event.
  const { data: events, error: eventsError } = useReportTargets(meetId);
  // What was unticked stays unticked while the app runs, per meet.
  const [skipped, setSkipped] = useViewState(`reports:skipped:${meetId}`, () => new Set<string>());
  const all = events ?? [];
  const chosen = all.filter((r) => !skipped.has(r.id)).map((r) => r.id);
  // Every event ticked prints them all (ids empty), also when what was unticked has since been deleted.
  const ids = chosen.length === all.length ? [] : chosen;
  const { data: teamScores, error: teamError } = useReportTeamScores(meetId, kind === "team_scores");
  const teamUnavailable = kind === "team_scores" && !teamScores?.rules.enabled;
  const sheet = kind === "checkin" || kind === "record" ? kind : null;
  const { data: bookHTML, error: bookError } = useReportHTML(sheet ? "startlist" : kind, meetId, ids, all.length, chosen.length, !sheet && !teamUnavailable);
  // 检录单 and 成绩记录表: by round, optionally one day.
  const { data: targets, error: targetsError } = useCheckInTargets(meetId);
  const [chosenDay, setDay] = useViewState(`reports:day:${meetId}`, "");
  const [skippedRounds, setSkippedRounds] = useViewState(`reports:skippedRounds:${meetId}`, () => new Set<string>());
  // A remembered day with no heats left lists every day.
  const day = targets && !targets.days.includes(chosenDay) ? "" : chosenDay;
  const rows = checkInRows(targets, day);
  const rounds = checkInSelection(rows, skippedRounds);
  const req = { meetId, roundIds: rounds.roundIds, day };
  const { data: sheetHTML, error: sheetError } = useSheetHTML(sheet ?? "checkin", req, sheet ? rounds.chosen.length : 0);

  const html = sheet ? sheetHTML : bookHTML;
  const count = teamUnavailable ? 0 : sheet ? rounds.chosen.length : chosen.length;
  const error = kind === "team_scores" && teamError ? teamError : sheet ? targetsError || sheetError : eventsError || bookError;
  // Why there is nothing to preview; "" while the choices are still loading.
  const listed = sheet ? rows.length : all.length;
  const none = teamUnavailable ? (teamScores ? "尚未启用团体计分" : "") : (sheet ? targets : events) === undefined ? "" : listed > 0 ? (sheet ? "未选择轮次" : "未选择项目") : !sheet ? "还没有项目" : day ? "当天没有组" : "还没有组";
  const print = async () => {
    setBusy(true);
    await (sheet ? ReportsController.printSheet(sheet, req) : ReportsController.print(kind, meetId, ids));
    setBusy(false);
  };
  return (
    <StPage flush
      toolbar={
        <StToolbar title="打印">
          <StSegmented value={kind} onChange={setKind}
            options={[{ value: "startlist", label: "秩序册" }, { value: "results", label: "成绩册" }, { value: "checkin", label: "检录单" }, { value: "record", label: "成绩记录表" }, { value: "team_scores", label: "团体总分" }]} />
          <StButton variant="primary" icon={<IconPrint size={16} strokeWidth={iconStroke} />} disabled={count === 0} loading={busy} onClick={print}>打印</StButton>
        </StToolbar>
      }>
      <div className="split">
        {sheet ? (
          <ChoiceList title="轮次" scrollKey={`reports:rounds:${meetId}`} items={rows} skipped={skippedRounds} onChange={setSkippedRounds}
            filter={
              <StSelect label="日期" width="fill" value={day} onChange={setDay}
                options={[{ value: "", label: "全部日期" }, ...(targets?.days ?? []).map((d) => ({ value: d, label: d }))]} />
            } />
        ) : (
          <ChoiceList title="项目" scrollKey={`reports:events:${meetId}`} items={all.map((e) => ({ id: e.id, label: e.name }))} skipped={skipped} onChange={setSkipped} />
        )}
        <div className="report-main">
          {error ? <div className="report-note"><StNotice tone="danger">{error}</StNotice></div> : count === 0 ? (none && <StEmpty>{none}</StEmpty>) : (
            <>
              <div className="subbar">
                <StSelect label="比例" size="small" value={zoom} onChange={setZoom} options={zooms} />
                <span className="subbar__spacer" />
                <span className="subbar__count">{pages > 0 && `${pages} 页`}</span>
              </div>
              <ReportPreview html={html} zoom={zoom} onPages={setPages} />
            </>
          )}
        </div>
      </div>
    </StPage>
  );
}

/** The side list of what to print: everything ticked unless unticked here. */
function ChoiceList({ title, scrollKey, items, skipped, onChange, filter }: {
  title: string;
  scrollKey: string;
  items: { id: string; label: string }[];
  skipped: ReadonlySet<string>;
  onChange: (next: Set<string>) => void;
  filter?: ReactNode;
}) {
  const list = useRef<HTMLDivElement>(null);
  useScrollMemory(list, scrollKey);
  const none = items.every((i) => !skipped.has(i.id));
  const toggle = (id: string) => {
    const next = new Set(skipped);
    if (!next.delete(id)) next.add(id);
    onChange(next);
  };
  return (
    <aside className="split__side">
      {filter && <div className="split__side-filter">{filter}</div>}
      <div className="split__side-head">
        <span>{title}</span>
        <StButton size="small" variant="ghost" reserveWidthFor={["全选", "取消全选"]}
          onClick={() => onChange(none ? new Set([...skipped, ...items.map((i) => i.id)]) : new Set([...skipped].filter((id) => !items.some((i) => i.id === id))))}>
          {none ? "取消全选" : "全选"}
        </StButton>
      </div>
      <div ref={list} className="split__side-list">
        {items.map((i) => (
          <StCheckbox key={i.id} checked={!skipped.has(i.id)} onChange={() => toggle(i.id)} label={i.label} />
        ))}
      </div>
    </aside>
  );
}
