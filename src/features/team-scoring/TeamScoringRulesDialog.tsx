import { useEffect, useRef, useState } from "react";
import { StButton, StDialog, StMuted, StSection, StSelect, StStack, StSwitchItem, StTable, StTextField } from "../../design";
import type { TeamScoreDetail, TeamScoreTarget, TeamScoringInput, TeamScoringPreview, UnitStanding } from "../../mygo";
import { TeamScoringController } from "./controller";

export function TeamScoringRulesDialog({ meetId, detail, onClose }: { meetId: string; detail: TeamScoreDetail; onClose: () => void }) {
  const [input, setInput] = useState<TeamScoringInput>(() => ({ enabled: detail.rules.enabled, places: detail.rules.places.join("、"), tiePolicy: detail.rules.tiePolicy, relayMultiplier: String(detail.rules.relayMultiplier), overrides: detail.rules.overrides.map((o) => ({ eventId: o.eventId ?? "", combinedId: o.combinedId ?? "", multiplier: String(o.multiplier) })) }));
  const [preview, setPreview] = useState<TeamScoringPreview | null>();
  const [busy, setBusy] = useState(false);
  const current = useRef({ input, detail });
  current.current = { input, detail };
  const review = useRef<HTMLDivElement>(null);
  useEffect(() => setPreview(undefined), [input, detail]);
  useEffect(() => { if (preview) review.current?.scrollIntoView({ block: "end" }); }, [preview]);
  const run = async () => {
    const at = current.current;
    setBusy(true);
    const p = await TeamScoringController.preview(meetId, input);
    setBusy(false);
    if (current.current.input === at.input && current.current.detail === at.detail && p) setPreview(p);
  };
  const save = async () => {
    if (!preview?.after) return;
    setBusy(true);
    const p = await TeamScoringController.apply({ meetId, rules: preview.after.rules }, preview.previewHash);
    setBusy(false);
    if (p) onClose(); else setPreview(undefined);
  };
  const multiplier = (t: TeamScoreTarget) => input.overrides.find((o) => o.eventId === t.eventId && o.combinedId === t.combinedId)?.multiplier ?? "";
  const changeMultiplier = (t: TeamScoreTarget, value: string) => setInput((i) => ({ ...i, overrides: [...i.overrides.filter((o) => o.eventId !== t.eventId || o.combinedId !== t.combinedId), ...(value === "" ? [] : [{ eventId: t.eventId, combinedId: t.combinedId, multiplier: value }])] }));
  return <StDialog open title="团体计分规则" size="large" onClose={() => !busy && onClose()} actions={<>
    <StButton disabled={busy} onClick={onClose}>取消</StButton>
    <StButton loading={busy} onClick={run}>预览重算</StButton>
    <StButton variant="primary" loading={busy} disabled={!preview?.changed} onClick={save}>保存规则</StButton>
  </>}>
    <div inert={busy}><StStack gap={16}>
      <StSwitchItem title="启用团体计分" checked={input.enabled} onChange={(enabled) => setInput((i) => ({ ...i, enabled }))} />
      <div className="form-grid">
        <StTextField label="名次分值" hint="从第一名起，用顿号分隔" width="fill" value={input.places} onChange={(places) => setInput((i) => ({ ...i, places }))} />
        <StTextField label="接力倍数" width="fill" inputMode="numeric" value={input.relayMultiplier} onChange={(relayMultiplier) => setInput((i) => ({ ...i, relayMultiplier }))} />
        <StSelect label="项目并列" width="fill" value={input.tiePolicy} onChange={(tiePolicy) => setInput((i) => ({ ...i, tiePolicy }))} options={[{ value: "average", label: "所占名次分值均分" }, { value: "same", label: "各得该名次分值" }]} />
      </div>
      <StSection title="项目例外" actions={<StMuted>留空沿用默认，0 不计分</StMuted>} padded={false}>
        <StTable<TeamScoreTarget> rows={detail.targets} rowKey={(r) => r.eventId || r.combinedId} empty="还没有项目" columns={[
          { key: "name", title: "项目", width: "minmax(200px, 1fr)", render: (r) => r.name },
          { key: "multiplier", title: "计分倍数", width: "140px", render: (r) => <StTextField aria-label={`${r.name}计分倍数`} width="fill" inputMode="numeric" placeholder={r.relay ? input.relayMultiplier : "1"} value={multiplier(r)} onChange={(v) => changeMultiplier(r, v)} /> },
        ]} />
      </StSection>
    </StStack></div>
    {preview?.after && <div ref={review}><StSection title="重算预览" actions={<StMuted>{preview.after.state}</StMuted>} padded={false}>
      <StTable<UnitStanding> rows={preview.after.rows} rowKey={(r) => r.unitId} empty="还没有参赛单位" columns={[
        { key: "unit", title: "参赛单位", width: "minmax(140px, 1fr)", render: (r) => r.unitName },
        { key: "before", title: "原总分", width: "96px", align: "end", render: (r) => preview.before?.rows.find((b) => b.unitId === r.unitId)?.total.text },
        { key: "after", title: "新总分", width: "96px", align: "end", render: (r) => r.total.text },
        { key: "rankBefore", title: "原名次", width: "88px", align: "end", render: (r) => preview.before?.rows.find((b) => b.unitId === r.unitId)?.rank ?? "—" },
        { key: "rankAfter", title: "新名次", width: "88px", align: "end", render: (r) => r.rank ?? "—" },
      ]} />
    </StSection></div>}
  </StDialog>;
}
