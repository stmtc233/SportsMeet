import { useState } from "react";
import { useNav } from "../../app/nav";
import { StButton, StDialog, StEmpty, StMuted, StNotice, StPage, StRow, StSection, StSegmented, StStack, StTable, StTag, StToolbar, type StColumn } from "../../design";
import type { TeamScoreBoard, TeamScoreLine, TeamScoreSource, UnitStanding } from "../../mygo";
import { useViewState } from "../../lib/viewState";
import { useGroups } from "../groups/controller";
import { GroupFilter, useGroupFilter } from "../groups/GroupFields";
import { TeamScoringController, toInput, useTeamBoards } from "./controller";
import { TeamScoringRulesDialog } from "./TeamScoringRulesDialog";

type Gender = "" | "男" | "女" | "混合";

export function TeamScoringPage({ meetId }: { meetId: string }) {
  const [gender, setGender] = useViewState<Gender>(`teamScores:gender:${meetId}`, "");
  const [group, setGroup] = useGroupFilter(meetId, "teamScores");
  const { data: groups } = useGroups(meetId);
  const { data: boards, error } = useTeamBoards({ meetId, gender });
  const [unit, setUnit] = useState<UnitStanding | null>(null);
  const [sources, setSources] = useState<TeamScoreBoard | null>(null);
  const [editing, setEditing] = useState(false);
  const [enabling, setEnabling] = useState(false);
  // Every board carries the meet's rules and targets.
  const detail = boards?.[0]?.detail ?? undefined;
  const enabled = detail?.rules.enabled ?? false;
  // A group without scoring events has nothing to rank.
  const shown = (boards ?? []).filter((b) => b.detail && b.detail.total > 0 && (group === "all" || b.groupId === group));

  const enable = async () => {
    if (!detail) return;
    setEnabling(true);
    await TeamScoringController.save(meetId, toInput(detail.rules, true), "已启用团体计分");
    setEnabling(false);
  };

  return (
    <StPage scrollKey={`teamScores:${meetId}`}
      toolbar={
        <StToolbar title="团体总分">
          <StButton disabled={!detail} onClick={() => setEditing(true)}>计分规则</StButton>
        </StToolbar>
      }>
      {error && <StNotice tone="danger">{error}</StNotice>}
      {detail && !enabled && (
        <StEmpty action={<StButton variant="primary" loading={enabling} onClick={enable}>启用团体计分</StButton>}>
          未启用团体计分
        </StEmpty>
      )}
      {detail && enabled && (
        <StStack gap={24}>
          <StRow>
            <StSegmented value={gender} onChange={setGender}
              options={[{ value: "", label: "全部" }, { value: "男", label: "男子" }, { value: "女", label: "女子" }, { value: "混合", label: "混合" }]} />
            {!!groups?.length && <GroupFilter meetId={meetId} value={group} onChange={setGroup} />}
          </StRow>
          {shown.length === 0 && <StEmpty>{gender || group !== "all" ? "没有符合条件的计分项目" : "还没有计分项目"}</StEmpty>}
          {shown.map((board) => (
            <Board key={board.groupId} board={board} titled={!!groups?.length} gender={gender}
              onUnit={setUnit} onSources={() => setSources(board)} />
          ))}
        </StStack>
      )}
      {unit && (
        <StDialog open title={unit.unitName} size="large" onClose={() => setUnit(null)}>
          <ScoreLines meetId={meetId} lines={unit.lines} />
        </StDialog>
      )}
      {sources?.detail && (
        <StDialog open title={sources.groupName ? `${sources.groupName}计分项目` : "计分项目"} size="large" onClose={() => setSources(null)}>
          <SourceList meetId={meetId} sources={sources.detail.sources} />
        </StDialog>
      )}
      {editing && detail && <TeamScoringRulesDialog meetId={meetId} detail={detail} onClose={() => setEditing(false)} />}
    </StPage>
  );
}

function Board({ board, titled, gender, onUnit, onSources }: { board: TeamScoreBoard; titled: boolean; gender: Gender; onUnit: (u: UnitStanding) => void; onSources: () => void }) {
  const data = board.detail!;
  const columns: StColumn<UnitStanding>[] = [
    { key: "rank", title: "名次", width: "56px", align: "end", render: (r) => r.rank ?? "" },
    { key: "unit", title: "参赛单位", width: "minmax(140px, 1fr)", render: (r) => <span className="cell-strong">{r.unitName}</span> },
    // With one gender chosen the other columns are all 0.
    ...(gender === ""
      ? [
          { key: "male", title: "男子", width: "80px", align: "end" as const, render: (r: UnitStanding) => r.male.text },
          { key: "female", title: "女子", width: "80px", align: "end" as const, render: (r: UnitStanding) => r.female.text },
          { key: "mixed", title: "混合", width: "80px", align: "end" as const, render: (r: UnitStanding) => r.mixed.text },
        ]
      : []),
    { key: "total", title: "总分", width: "88px", align: "end", render: (r) => <strong>{r.total.text}</strong> },
  ];
  return (
    <StStack gap={12}>
      {data.warnings.map((w) => <StNotice key={w} tone="warning">{w}</StNotice>)}
      <StSection padded={false}
        title={<>{titled ? board.groupName : "单位排名"} <StTag tone={data.complete ? "success" : "neutral"}>{data.state}</StTag></>}
        actions={<StButton size="small" variant="ghost" onClick={onSources}>已计分 {data.counted} / {data.total} 项</StButton>}>
        <div className="team-score-table">
          <StTable<UnitStanding> rows={data.rows} rowKey={(r) => r.unitId} onRowClick={onUnit} empty="还没有参赛单位" columns={columns} />
        </div>
      </StSection>
      {data.unassigned.length > 0 && (
        <StSection title="无参赛单位" padded={false}>
          <ScoreLines lines={data.unassigned} />
        </StSection>
      )}
    </StStack>
  );
}

function SourceList({ meetId, sources }: { meetId: string; sources: TeamScoreSource[] }) {
  const { go } = useNav();
  return (
    <div className="preview-table">
      <StTable<TeamScoreSource> rows={sources} rowKey={(r) => r.eventId || r.combinedId} empty="没有计分项目" dense
        onRowClick={(r) => go(r.combinedId ? { page: "combined", meetId, combinedId: r.combinedId } : { page: "event", meetId, eventId: r.eventId, tab: "results" })}
        columns={[
          { key: "name", title: "项目", width: "minmax(180px, 1fr)", render: (r) => r.name },
          { key: "multiplier", title: "倍数", width: "64px", align: "end", render: (r) => r.multiplier },
          { key: "state", title: "状态", width: "96px", render: (r) => <StTag tone={r.complete ? "success" : "neutral"}>{r.state}</StTag> },
        ]} />
    </div>
  );
}

function ScoreLines({ meetId, lines }: { meetId?: string; lines: TeamScoreLine[] }) {
  const { go } = useNav();
  return (
    <div className="team-score-table">
      <StTable<TeamScoreLine> rows={lines} rowKey={(r) => `${r.eventId || r.combinedId}:${r.competitorId}`} empty="还没有得分" dense
        onRowClick={meetId ? (r) => go(r.combinedId ? { page: "combined", meetId, combinedId: r.combinedId } : { page: "event", meetId, eventId: r.eventId, tab: "results" }) : undefined}
        columns={[
          { key: "event", title: "项目", width: "minmax(160px, 1fr)", render: (r) => r.eventName },
          { key: "name", title: "运动员 / 队伍", width: "minmax(96px, 0.6fr)", render: (r) => r.name },
          { key: "rank", title: "名次", width: "56px", align: "end", render: (r) => r.rank ?? "" },
          { key: "base", title: "名次分", width: "72px", align: "end", render: (r) => <StMuted>{r.multiplier === 1 ? r.base.text : `${r.base.text} × ${r.multiplier}`}</StMuted> },
          { key: "score", title: "得分", width: "72px", align: "end", render: (r) => <strong>{r.score.text}</strong> },
        ]} />
    </div>
  );
}
