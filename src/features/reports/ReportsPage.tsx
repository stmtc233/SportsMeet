import { useEffect, useState, type ReactNode } from "react";
import { IconPrint, StButton, StCheckbox, StPage, StSegmented, StSelect, StToolbar, iconStroke } from "../../design";
import { checkInRows, checkInSelection } from "./checkin";
import { ReportsController, useCheckInHTML, useCheckInTargets, useReportHTML, useReportTargets } from "./controller";

type Kind = "startlist" | "results" | "checkin";

/** 秩序册, 成绩册 and 检录单: rendered by Go, previewed in a frame, printed from a window of their own. */
export function ReportsPage({ meetId }: { meetId: string }) {
  const [kind, setKind] = useState<Kind>("startlist");
  const [busy, setBusy] = useState(false);
  // 秩序册 and 成绩册: by event.
  const { data: events } = useReportTargets(meetId);
  const [skipped, setSkipped] = useState(new Set<string>());
  const all = events ?? [];
  const chosen = all.filter((r) => !skipped.has(r.id)).map((r) => r.id);
  const ids = skipped.size === 0 ? [] : chosen;
  const book = kind === "checkin" ? "startlist" : kind;
  const { data: bookHTML } = useReportHTML(book, meetId, ids, all.length, chosen.length, kind !== "checkin");
  // 检录单: by round, optionally one day.
  const { data: targets } = useCheckInTargets(meetId);
  const [day, setDay] = useState("");
  const [skippedRounds, setSkippedRounds] = useState(new Set<string>());
  const rows = checkInRows(targets, day);
  const rounds = checkInSelection(rows, skippedRounds);
  const req = { meetId, roundIds: rounds.roundIds, day };
  const { data: checkInHTML } = useCheckInHTML(req, kind === "checkin" ? rounds.chosen.length : 0);
  useEffect(() => {
    setSkipped(new Set());
    setSkippedRounds(new Set());
    setDay("");
  }, [meetId]);

  const checkIn = kind === "checkin";
  const html = checkIn ? checkInHTML : bookHTML;
  const count = checkIn ? rounds.chosen.length : chosen.length;
  const print = async () => {
    setBusy(true);
    await (checkIn ? ReportsController.printCheckIn(req) : ReportsController.print(kind, meetId, ids));
    setBusy(false);
  };
  return (
    <StPage flush
      toolbar={
        <StToolbar title="打印">
          <StSegmented value={kind} onChange={setKind}
            options={[{ value: "startlist", label: "秩序册" }, { value: "results", label: "成绩册" }, { value: "checkin", label: "检录单" }]} />
          <StButton variant="primary" icon={<IconPrint size={16} strokeWidth={iconStroke} />} disabled={count === 0} loading={busy} onClick={print}>打印</StButton>
        </StToolbar>
      }>
      <div className="split">
        {checkIn ? (
          <ChoiceList title="轮次" items={rows} skipped={skippedRounds} onChange={setSkippedRounds}
            filter={
              <StSelect label="日期" width="fill" value={day} onChange={setDay}
                options={[{ value: "", label: "全部日期" }, ...(targets?.days ?? []).map((d) => ({ value: d, label: d }))]} />
            } />
        ) : (
          <ChoiceList title="项目" items={all.map((e) => ({ id: e.id, label: e.name }))} skipped={skipped} onChange={setSkipped} />
        )}
        <iframe className="report-frame" title="预览" sandbox="" srcDoc={html ?? ""} />
      </div>
    </StPage>
  );
}

/** The side list of what to print: everything ticked unless unticked here. */
function ChoiceList({ title, items, skipped, onChange, filter }: {
  title: string;
  items: { id: string; label: string }[];
  skipped: ReadonlySet<string>;
  onChange: (next: Set<string>) => void;
  filter?: ReactNode;
}) {
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
      <div className="split__side-list">
        {items.map((i) => (
          <StCheckbox key={i.id} checked={!skipped.has(i.id)} onChange={() => toggle(i.id)} label={i.label} />
        ))}
      </div>
    </aside>
  );
}
