import { useEffect, useState } from "react";
import { useNav } from "../../app/nav";
import { StButton, StDialog, StEmpty, StMuted, StNotice, StPage, StRow, StSection, StSelect, StStack, StTable, StTag, StToolbar } from "../../design";
import { useViewState } from "../../lib/viewState";
import type { TeamScoreLine, TeamScoreSource, UnitStanding } from "../../mygo";
import { useTeamScores } from "./controller";
import { TeamScoringRulesDialog } from "./TeamScoringRulesDialog";

export function TeamScoringPage({ meetId }: { meetId: string }) {
  const [gender, setGender] = useViewState(`teamScores:gender:${meetId}`, "");
  const [group, setGroup] = useViewState(`teamScores:group:${meetId}`, "all");
  const [selectedUnit, setSelectedUnit] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const { data, error } = useTeamScores({ meetId, gender, ...(group === "all" ? {} : { groupLabel: group.slice(1) }) });
  useEffect(() => { if (data && group !== "all" && !data.groups.includes(group.slice(1))) setGroup("all"); }, [data, group, setGroup]);
  const row = data?.rows.find((r) => r.unitId === selectedUnit);
  return <StPage scrollKey={`teamScores:${meetId}`} toolbar={<StToolbar title="团体总分"><StButton onClick={() => setEditing(true)} disabled={!data}>计分规则</StButton></StToolbar>}>
    {error && <StNotice tone="danger">{error}</StNotice>}
    {data && (!data.rules.enabled ? <StEmpty action={<StButton onClick={() => setEditing(true)}>设置计分规则</StButton>}>尚未启用团体计分</StEmpty> : <StStack gap={24}>
      <StRow>
        <StSelect label="性别" value={gender} onChange={setGender} options={[{ value: "", label: "全部性别" }, { value: "男", label: "男子" }, { value: "女", label: "女子" }, { value: "混合", label: "混合" }]} />
        <StSelect label="组别" value={group} onChange={setGroup} options={[{ value: "all", label: "全部组别" }, ...data.groups.map((g) => ({ value: `g${g}`, label: g || "无组别" }))]} />
      </StRow>
      <StSection title={data.state} actions={<StMuted>已计分 {data.counted} / {data.total} 项</StMuted>} padded={false}>
        <div className="team-score-table"><StTable<UnitStanding> rows={data.rows} rowKey={(r) => r.unitId} empty="还没有参赛单位" columns={[
          { key: "rank", title: "名次", width: "56px", align: "end", render: (r) => r.rank ?? "—" },
          { key: "unit", title: "参赛单位", width: "minmax(112px, 1fr)", render: (r) => <StButton variant="ghost" size="small" title={r.unitName} onClick={() => setSelectedUnit(r.unitId)}>{r.unitName}</StButton> },
          { key: "male", title: "男子得分", width: "88px", align: "end", render: (r) => r.male.text },
          { key: "female", title: "女子得分", width: "88px", align: "end", render: (r) => r.female.text },
          { key: "mixed", title: "混合得分", width: "88px", align: "end", render: (r) => r.mixed.text },
          { key: "total", title: "总分", width: "88px", align: "end", render: (r) => <strong>{r.total.text}</strong> },
        ]} /></div>
      </StSection>
      <StSection title="项目计分" padded={false}><StTable<TeamScoreSource> rows={data.sources} rowKey={(r) => r.eventId || r.combinedId} empty="没有符合条件的项目" columns={[
        { key: "event", title: "项目", width: "minmax(220px, 1fr)", render: (r) => r.name },
        { key: "multiplier", title: "倍数", width: "100px", align: "end", render: (r) => r.multiplier },
        { key: "state", title: "状态", width: "120px", render: (r) => <StTag tone={r.state === "已计分" ? "success" : "neutral"}>{r.state}</StTag> },
      ]} /></StSection>
      {data.unassigned.length > 0 && <StSection title="无参赛单位" padded={false}><ScoreLines meetId={meetId} lines={data.unassigned} /></StSection>}
    </StStack>)}
    {row && <StDialog open title={row.unitName} size="large" onClose={() => setSelectedUnit(null)} actions={<StButton onClick={() => setSelectedUnit(null)}>关闭</StButton>}>
      <ScoreLines meetId={meetId} lines={row.lines} />
    </StDialog>}
    {editing && data && <TeamScoringRulesDialog meetId={meetId} detail={data} onClose={() => setEditing(false)} />}
  </StPage>;
}

function ScoreLines({ meetId, lines }: { meetId: string; lines: TeamScoreLine[] }) {
  const { go } = useNav();
  return <div className="team-score-table"><StTable<TeamScoreLine> rows={lines} rowKey={(r) => `${r.eventId || r.combinedId}:${r.competitorId}`} empty="尚无得分明细" columns={[
    { key: "event", title: "项目", width: "minmax(180px, 1fr)", render: (r) => <StButton size="small" variant="ghost" onClick={() => go(r.combinedId ? { page: "combined", meetId, combinedId: r.combinedId } : { page: "event", meetId, eventId: r.eventId, tab: "results" })}>{r.eventName}</StButton> },
    { key: "name", title: "运动员／队伍", width: "140px", render: (r) => r.name },
    { key: "rank", title: "名次", width: "64px", align: "end", render: (r) => r.rank },
    { key: "base", title: "基础分", width: "88px", align: "end", render: (r) => r.base.text },
    { key: "multiplier", title: "倍数", width: "64px", align: "end", render: (r) => r.multiplier },
    { key: "score", title: "团体得分", width: "100px", align: "end", render: (r) => r.score.text },
  ]} /></div>;
}
