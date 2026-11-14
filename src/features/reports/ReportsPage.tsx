import { Fragment, useEffect, useRef, useState, type ReactNode } from "react";
import { IconPrint, StButton, StCheckbox, StEmpty, StListItem, StMultiSelect, StNavGroup, StNavHeading, StNotice, StPage, StSelect, StToolbar, iconStroke } from "../../design";
import { recall, remember, useScrollMemory, useViewState } from "../../lib/viewState";
import { useGroups } from "../groups/controller";
import { ungrouped } from "../groups/GroupFields";
import type { Zoom } from "./paginate";
import { ReportPreview, type PrintPages } from "./ReportPreview";
import { checkInRows, checkInSelection } from "./checkin";
import { ReportsController, useCheckInTargets, useReportHTML, useReportTargets, useReportTeamScores, useSheetHTML } from "./controller";
import { reportGroups, reportKindKey, reportKinds, reportScopeKey, type ReportKind as Kind, type ReportScope } from "./kinds";

const zooms: { value: Zoom; label: string }[] = [
  { value: "fit", label: "适合宽度" }, { value: "0.5", label: "50%" }, { value: "0.75", label: "75%" }, { value: "1", label: "100%" }, { value: "1.5", label: "150%" },
];

/** Reports rendered by Go, laid out on A4 sheets here, then printed from a window of their own. */
export function ReportsPage({ meetId }: { meetId: string }) {
  const [savedKind, setKind] = useViewState<Kind>(reportKindKey, "startlist", true);
  const kind = reportKinds.some((k) => k.value === savedKind) ? savedKind : "startlist";
  const [busy, setBusy] = useState(false);
  const [savedZoom, setZoom] = useViewState<Zoom>("reports:zoom", "fit", true);
  const zoom = zooms.some((z) => z.value === savedZoom) ? savedZoom : "fit";
  const [pages, setPages] = useState(0);
  // The report on the sheets shown; printing waits for the newest one.
  const [shown, setShown] = useState<string>();
  const printPages = useRef<PrintPages | null>(null);
  // Books and 报名表: by event.
  const { data: events, error: eventsError } = useReportTargets(meetId);
  // What was unticked stays unticked while the app runs, per meet.
  const [skipped, setSkipped] = useViewState(`reports:skipped:${meetId}`, () => new Set<string>());
  const { data: groups } = useGroups(meetId);
  // 成绩册 and 团体总分 print group by group, each from a new page; none picked prints all.
  const grouped = (kind === "results" || kind === "team_scores") && !!groups?.length;
  const groupChoices = [...(groups ?? []).map((g) => ({ value: g.id, label: g.name })), ...((events ?? []).some((e) => !e.groupId) ? [{ value: "", label: ungrouped }] : [])];
  const [pickedGroups, setPickedGroups] = useViewState<string[]>(`reports:groups:${meetId}`, []);
  // A group deleted meanwhile drops out of the selection.
  const groupIds = grouped ? pickedGroups.filter((id) => groupChoices.some((c) => c.value === id)) : [];
  const all = (events ?? []).filter((e) => !groupIds.length || groupIds.includes(e.groupId ?? ""));
  const chosen = all.filter((r) => !skipped.has(r.id)).map((r) => r.id);
  // Every event ticked prints them all (ids empty), also when what was unticked has since been deleted.
  const ids = chosen.length === all.length ? [] : chosen;
  const { data: teamScores, error: teamError } = useReportTeamScores(meetId, kind === "team_scores");
  const teamUnavailable = kind === "team_scores" && !teamScores?.rules.enabled;
  const sheet = kind === "checkin" || kind === "record" ? kind : null;
  const { data: bookHTML, error: bookError } = useReportHTML(sheet ? "startlist" : kind, meetId, ids, all.length, chosen.length, !sheet && !teamUnavailable, groupIds);
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

  // Opened from another page for some rounds or events: tick just those, once.
  const scopeKey = reportScopeKey(meetId);
  useEffect(() => {
    const scope = recall<ReportScope>(scopeKey);
    if (!scope || scope.kind !== kind) return;
    const ids = new Set(scope.ids);
    if (sheet) {
      if (!targets) return;
      setDay("");
      setSkippedRounds(new Set(checkInRows(targets, "").map((r) => r.id).filter((id) => !ids.has(id))));
    } else {
      if (!events) return;
      setPickedGroups([]);
      setSkipped(new Set(events.map((e) => e.id).filter((id) => !ids.has(id))));
    }
    remember(scopeKey, undefined);
  }, [scopeKey, kind, sheet, targets, events, setDay, setSkippedRounds, setSkipped, setPickedGroups]);

  const html = sheet ? sheetHTML : bookHTML;
  const count = teamUnavailable ? 0 : sheet ? rounds.chosen.length : chosen.length;
  const error = kind === "team_scores" && teamError ? teamError : sheet ? targetsError || sheetError : eventsError || bookError;
  // Why there is nothing to preview; "" while the choices are still loading.
  const listed = sheet ? rows.length : all.length;
  const none = teamUnavailable ? (teamScores ? "尚未启用团体计分" : "") : (sheet ? targets : events) === undefined ? "" : listed > 0 ? (sheet ? "未选择轮次" : "未选择项目") : !sheet ? "还没有项目" : day ? "当天没有组" : "还没有组";
  const print = async () => {
    const doc = printPages.current?.();
    if (!doc) return;
    setBusy(true);
    await ReportsController.printPages(doc.title, doc.html);
    setBusy(false);
  };
  return (
    <StPage flush
      toolbar={
        <StToolbar title="打印">
          <StButton variant="primary" icon={<IconPrint size={16} strokeWidth={iconStroke} />} disabled={count === 0 || !html || shown !== html} loading={busy} onClick={print}>打印</StButton>
        </StToolbar>
      }>
      <div className="split">
        {sheet ? (
          <ChoiceList kind={kind} onKind={setKind} title="轮次" scrollKey={`reports:rounds:${meetId}`} items={rows} skipped={skippedRounds} onChange={setSkippedRounds}
            filter={
              <StSelect label="日期" width="fill" value={day} onChange={setDay}
                options={[{ value: "", label: "全部日期" }, ...(targets?.days ?? []).map((d) => ({ value: d, label: d }))]} />
            } />
        ) : (
          <ChoiceList kind={kind} onKind={setKind} title="项目" scrollKey={`reports:events:${meetId}`} items={all.map((e) => ({ id: e.id, label: e.name }))} skipped={skipped} onChange={setSkipped}
            filter={grouped ? <StMultiSelect aria-label="组别" width="fill" allLabel="全部组别" value={groupIds} options={groupChoices} onChange={setPickedGroups} /> : undefined} />
        )}
        <div className="report-main">
          {error ? <div className="report-note"><StNotice tone="danger">{error}</StNotice></div> : count === 0 ? (none && <StEmpty>{none}</StEmpty>) : (
            <>
              <div className="subbar">
                <StSelect label="比例" size="small" value={zoom} onChange={setZoom} options={zooms} />
                <span className="subbar__spacer" />
                <span className="subbar__count">{pages > 0 && `${pages} 页`}</span>
              </div>
              <ReportPreview html={html} zoom={zoom} printRef={printPages}
                onPages={(n, h) => { setPages(n); setShown(h); }} />
            </>
          )}
        </div>
      </div>
    </StPage>
  );
}

/**
 * The side of 打印: the reports by part of the meet, then what to print of
 * the one chosen, everything ticked unless unticked here.
 */
function ChoiceList({ kind, onKind, title, scrollKey, items, skipped, onChange, filter }: {
  kind: Kind;
  onKind: (kind: Kind) => void;
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
      <nav className="report-kinds" aria-label="报表">
        <StNavGroup>
          {reportGroups.map((g) => (
            <Fragment key={g.title}>
              <StNavHeading>{g.title}</StNavHeading>
              {g.kinds.map((k) => <StListItem key={k} title={reportKinds.find((r) => r.value === k)!.label} selected={k === kind} onClick={() => onKind(k)} />)}
            </Fragment>
          ))}
        </StNavGroup>
      </nav>
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
