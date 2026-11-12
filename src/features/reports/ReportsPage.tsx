import { useEffect, useState } from "react";
import { IconPrint, StButton, StCheckbox, StPage, StSegmented, StToolbar, iconStroke } from "../../design";
import { ReportsController, useReportHTML, useReportTargets } from "./controller";

/** 秩序册 and 成绩册: rendered by Go, previewed in a frame, printed from a window of their own. */
export function ReportsPage({ meetId }: { meetId: string }) {
  const { data: events } = useReportTargets(meetId);
  const [kind, setKind] = useState("startlist");
  const [skipped, setSkipped] = useState(new Set<string>());
  const [busy, setBusy] = useState(false);
  const all = events ?? [];
  const chosen = all.filter((r) => !skipped.has(r.id)).map((r) => r.id);
  const ids = skipped.size === 0 ? [] : chosen;
  const { data: html } = useReportHTML(kind, meetId, ids, all.length, chosen.length);
  useEffect(() => setSkipped(new Set()), [meetId]);
  const toggle = (id: string) => setSkipped((old) => {
    const next = new Set(old);
    if (!next.delete(id)) next.add(id);
    return next;
  });
  const print = async () => {
    setBusy(true);
    await ReportsController.print(kind, meetId, ids);
    setBusy(false);
  };
  return (
    <StPage flush
      toolbar={
        <StToolbar title="打印">
          <StSegmented value={kind} onChange={setKind} options={[{ value: "startlist", label: "秩序册" }, { value: "results", label: "成绩册" }]} />
          <StButton variant="primary" icon={<IconPrint size={16} strokeWidth={iconStroke} />} disabled={chosen.length === 0} loading={busy} onClick={print}>打印</StButton>
        </StToolbar>
      }>
      <div className="split">
        <aside className="split__side">
          <div className="split__side-head">
            <span>项目</span>
            <StButton size="small" variant="ghost" reserveWidthFor={["全选", "取消全选"]} onClick={() => setSkipped(skipped.size === 0 ? new Set(all.map((r) => r.id)) : new Set())}>
              {skipped.size === 0 ? "取消全选" : "全选"}
            </StButton>
          </div>
          <div className="split__side-list">
            {all.map((r) => (
              <StCheckbox key={r.id} checked={!skipped.has(r.id)} onChange={() => toggle(r.id)} label={r.name} />
            ))}
          </div>
        </aside>
        <iframe className="report-frame" title="预览" sandbox="" srcDoc={html ?? ""} />
      </div>
    </StPage>
  );
}
