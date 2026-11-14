import { useState } from "react";
import { IconChevronRight, StListItem, StSection, StSwitchItem, iconStroke } from "../../design";
import { TeamScoringController, toInput, useTeamDetail } from "./controller";
import { TeamScoringRulesDialog } from "./TeamScoringRulesDialog";

/** 团体总分 in 赛事设置: whether units are scored, and by which rules. */
export function TeamScoringSection({ meetId }: { meetId: string }) {
  const { data: detail } = useTeamDetail(meetId);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const rules = detail?.rules;
  const enable = async (enabled: boolean) => {
    if (!detail) return;
    setBusy(true);
    await TeamScoringController.save(meetId, toInput(detail.rules, enabled), enabled ? "已启用团体计分" : "已停用团体计分");
    setBusy(false);
  };
  return (
    <StSection title="团体总分" padded={false}>
      <StSwitchItem title="启用团体计分" checked={rules?.enabled ?? false} disabled={!detail || busy} onChange={enable} />
      <StListItem title="计分规则" subtitle={rules ? `名次分 ${rules.places.join("、")}` : " "} disabled={!detail}
        trailing={<IconChevronRight size={16} strokeWidth={iconStroke} />} onClick={() => setEditing(true)} />
      {editing && detail && <TeamScoringRulesDialog meetId={meetId} detail={detail} onClose={() => setEditing(false)} />}
    </StSection>
  );
}
