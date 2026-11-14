import { useState } from "react";
import { IconImport, StButton, StMenu, StPage, StToolbar, iconStroke } from "../../design";
import { useViewState } from "../../lib/viewState";
import type { EntryForm } from "../../mygo";
import { AthletesView } from "../athletes/AthletesView";
import { useAthletes } from "../athletes/controller";
import { ImportDialog } from "../athletes/ImportDialog";
import { EntriesController, useEntrySummary } from "./controller";
import { EntryFormDialog } from "./EntryFormDialog";
import { RosterTree } from "./RosterTree";
import { allNode, exportChoices, knownNode, nodeKey, nodePath, parseNode, rosterDetailKey, rosterNodeKey, treeRows, type RosterNode } from "./tree";

/**
 * 报名: the tree of 组别 → 参赛单位 on the left with each unit's 报名进度,
 * the athletes of the picked node on the right; hands out the 报名登记表
 * and takes them back.
 */
export function EntriesPage({ meetId }: { meetId: string }) {
  const { data: summary } = useEntrySummary(meetId);
  const { data: athletes } = useAthletes(meetId);
  const [savedNode, setNode] = useViewState(rosterNodeKey(meetId), nodeKey(allNode));
  const [detail, setDetail] = useViewState(rosterDetailKey(meetId), false);
  const [forms, setForms] = useState<EntryForm[] | null>(null);
  const [importing, setImporting] = useState(false);

  // A remembered node whose group or unit is gone shows everyone.
  const node = summary ? knownNode(treeRows(summary), parseNode(savedNode)) : parseNode(savedNode);
  const units = summary?.units ?? [];
  const entered = units.filter((u) => u.entries > 0 || u.teams > 0).length;
  const choices = summary ? exportChoices(summary, node) : [];
  const select = (n: RosterNode) => {
    setNode(nodeKey(n));
    setDetail(true);
  };
  const pickForms = async () => {
    const picked = await EntriesController.pickForms();
    if (picked.length > 0) setForms(picked);
  };
  const exportButton = (props: object = {}) => <StButton disabled={choices.length === 0} {...props}>导出登记表</StButton>;

  return (
    <StPage flush
      toolbar={
        <StToolbar title="报名" subtitle={units.length ? `${entered} / ${units.length} 单位已报名` : ""}>
          <StButton icon={<IconImport size={16} strokeWidth={iconStroke} />} onClick={() => setImporting(true)}>导入名单</StButton>
          {choices.length > 1
            ? <StMenu trigger={(p) => exportButton(p)} items={choices.map((c) => ({ label: c.label, onSelect: () => void EntriesController.exportForms(meetId, c.ids, c.byGroup) }))} />
            : exportButton({ onClick: () => choices[0] && void EntriesController.exportForms(meetId, choices[0].ids, choices[0].byGroup) })}
          <StButton variant="primary" disabled={!units.length} onClick={pickForms}>导入登记表</StButton>
        </StToolbar>
      }>
      <div className="split roster" data-detail={detail || undefined}>
        <RosterTree meetId={meetId} node={node} total={athletes?.length} onSelect={select} />
        <AthletesView meetId={meetId} node={node} path={summary ? nodePath(summary, node) : []} onBack={() => setDetail(false)} />
      </div>
      <EntryFormDialog meetId={meetId} forms={forms} onPick={pickForms} onClose={() => setForms(null)} />
      <ImportDialog meetId={meetId} open={importing} onClose={() => setImporting(false)} />
    </StPage>
  );
}
