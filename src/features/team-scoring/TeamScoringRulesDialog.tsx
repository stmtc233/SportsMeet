import { useState } from "react";
import { StButton, StDialog, StSegmented, StStack, StSwitchItem, StTable, StTextField } from "../../design";
import type { TeamScoreDetail, TeamScoreTarget, TeamScoringInput } from "../../mygo";
import { TeamScoringController, toInput } from "./controller";

const sameTarget = (a: { eventId?: string; combinedId?: string }, b: TeamScoreTarget) => (a.eventId ?? "") === b.eventId && (a.combinedId ?? "") === b.combinedId;

/** Edits the rules; saving recalculates the standings on the page at once. */
export function TeamScoringRulesDialog({ meetId, detail, onClose }: { meetId: string; detail: TeamScoreDetail; onClose: () => void }) {
  const [input, setInput] = useState<TeamScoringInput>(() => toInput(detail.rules));
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<TeamScoringInput>) => setInput((i) => ({ ...i, ...patch }));
  const multiplier = (t: TeamScoreTarget) => input.overrides.find((o) => sameTarget(o, t))?.multiplier ?? "";
  // Empty follows the default multiplier, so it is not stored.
  const setMultiplier = (t: TeamScoreTarget, value: string) =>
    set({ overrides: [...input.overrides.filter((o) => !sameTarget(o, t)), ...(value.trim() === "" ? [] : [{ eventId: t.eventId, combinedId: t.combinedId, multiplier: value }])] });

  const save = async () => {
    setBusy(true);
    const ok = await TeamScoringController.save(meetId, input);
    setBusy(false);
    if (ok) onClose();
  };

  return (
    <StDialog open title="计分规则" size="large" onClose={() => !busy && onClose()} onSubmit={save}
      actions={
        <>
          <StButton disabled={busy} onClick={onClose}>取消</StButton>
          <StButton variant="primary" loading={busy} type="submit">保存</StButton>
        </>
      }>
      <StStack gap={16}>
        <StSwitchItem title="启用团体计分" checked={input.enabled} onChange={(enabled) => set({ enabled })} />
        <div className="form-grid">
          <div className="span">
            <StTextField label="名次分值" hint="从第一名起，用顿号分隔" width="fill" value={input.places} onChange={(places) => set({ places })} />
          </div>
          <StTextField label="接力倍数" width="fill" inputMode="numeric" value={input.relayMultiplier} onChange={(relayMultiplier) => set({ relayMultiplier })} />
          <StSegmented label="名次并列" value={input.tiePolicy} onChange={(tiePolicy) => set({ tiePolicy })}
            options={[{ value: "average", label: "分值均分" }, { value: "same", label: "各得全分" }]} />
        </div>
        <div className="preview-table">
          <StTable<TeamScoreTarget> rows={detail.targets} rowKey={(r) => r.eventId || r.combinedId} empty="还没有项目" columns={[
            { key: "name", title: "项目", width: "minmax(180px, 1fr)", render: (r) => r.name },
            { key: "multiplier", title: "倍数", width: "120px", render: (r) => (
              <StTextField aria-label={`${r.name}的倍数`} width="fill" inputMode="numeric" align="end" placeholder={r.relay ? input.relayMultiplier : "1"}
                value={multiplier(r)} onChange={(v) => setMultiplier(r, v)} />
            ) },
          ]} />
        </div>
      </StStack>
    </StDialog>
  );
}
