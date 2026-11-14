import { useEffect, useMemo, useRef, useState } from "react";
import { StButton, StDialog, StSection, StTable } from "../../design";
import type { CombinedDetail, CombinedRow, CombinedRulesPreview } from "../../mygo";
import { CombinedController, inputFromEvent } from "./controller";
import { PointRuleEditor } from "./PointRuleEditor";

export function CombinedRulesDialog({ detail, onClose }: { detail: CombinedDetail; onClose: () => void }) {
  const events = useMemo(() => detail.components.map((c) => inputFromEvent(c.event)), [detail.components]);
  const [rules, setRules] = useState(() => detail.components.map((c) => ({ eventId: c.event.id, rule: c.input })));
  const [preview, setPreview] = useState<CombinedRulesPreview | null>();
  const review = useRef<HTMLDivElement>(null);
  useEffect(() => { if (preview) review.current?.scrollIntoView({ block: "end" }); }, [preview]);
  const [busy, setBusy] = useState(false);
  const request = useMemo(() => ({ combinedId: detail.combined.id, rules }), [detail.combined.id, rules]);
  useEffect(() => setPreview(undefined), [request, detail]);
  const run = async () => { setBusy(true); const p = await CombinedController.previewRules(request); setBusy(false); if (p) setPreview(p); };
  const save = async () => {
    if (!preview) return;
    setBusy(true); const p = await CombinedController.applyRules(request, preview.previewHash); setBusy(false);
    if (p) onClose(); else setPreview(undefined);
  };
  return <StDialog open title="计分规则" size="large" onClose={() => !busy && onClose()}
    actions={<><StButton disabled={busy} onClick={onClose}>取消</StButton><StButton loading={busy} onClick={run}>预览重算</StButton><StButton variant="primary" loading={busy} disabled={!preview?.changed} onClick={save}>保存规则</StButton></>}>
    <div inert={busy}>
      {rules.map((r, i) => <StSection key={r.eventId} title={detail.components[i]?.event.discipline ?? ""}><div className="form-grid">
        <PointRuleEditor event={events[i]!} value={r.rule} onChange={(rule) => setRules((old) => old.map((p, n) => n === i ? { ...p, rule } : p))} />
      </div></StSection>)}
    </div>
    {preview && <div ref={review} className="view-enter"><StSection title="重算预览" padded={false}><StTable<CombinedRow> rows={preview.after} rowKey={(r) => r.athleteId} dense columns={[
      { key: "name", title: "姓名", render: (r) => r.name },
      { key: "before", title: "原总分", align: "end", render: (r) => preview.before.find((b) => b.athleteId === r.athleteId)?.total ?? "" },
      { key: "after", title: "新总分", align: "end", render: (r) => r.total },
      { key: "rankBefore", title: "原名次", align: "end", render: (r) => preview.before.find((b) => b.athleteId === r.athleteId)?.rank ?? "—" },
      { key: "rankAfter", title: "新名次", align: "end", render: (r) => r.excluded ? "不列名次" : r.rank ?? "—" },
    ]} /></StSection></div>}
  </StDialog>;
}
