import { useEffect, useState } from "react";
import { StButton, StSelect, StTable, StTextArea } from "../../design";
import type { EventInput, PointBandView, PointRuleInput, PointRulePreview } from "../../mygo";
import { CombinedController } from "./controller";

export function PointRuleEditor({ event, value, onChange }: { event: EventInput; value: PointRuleInput; onChange: (rule: PointRuleInput) => void }) {
  const [preview, setPreview] = useState<PointRulePreview | null>();
  const [busy, setBusy] = useState(false);
  useEffect(() => setPreview(undefined), [value, event]);
  const run = async () => { setBusy(true); const p = await CombinedController.parseRule(event, value); setBusy(false); if (p) setPreview(p); };
  return <>
    <div className="span"><StTextArea label="计分表" placeholder={"成绩\t积分"} value={value.table} rows={4} onChange={(table) => onChange({ ...value, table })} /></div>
    {([ ["DNS", "弃权"], ["DNF", "未完成"], ["DQ", "犯规取消"], ["NM", "无有效成绩"] ] as const).map(([status, label]) => <StSelect key={status} label={label} value={value.invalidPolicies[status] ?? ""}
      onChange={(policy) => onChange({ ...value, invalidPolicies: { ...value.invalidPolicies, [status]: policy } })}
      options={[{ value: "zero", label: "该项零分" }, { value: "exclude", label: "不列总排名" }]} />)}
    <div className="span"><StButton loading={busy} onClick={run}>预览计分表</StButton></div>
    {preview && <div className="span view-enter"><StTable<PointBandView> rows={preview.rows} rowKey={(r) => r.markText} dense columns={[{ key: "mark", title: "成绩标准", render: (r) => r.markText }, { key: "points", title: "积分", align: "end", render: (r) => r.points }]} /></div>}
  </>;
}
