import { useState } from "react";
import { useNav } from "../../app/nav";
import { IconBack, StButton, StEmpty, StIconButton, StNotice, StPage, StSection, StSegmented, StTable, StTag, StToolbar, iconStroke } from "../../design";
import type { CombinedComponentView, CombinedRow } from "../../mygo";
import { kindLabels } from "../../lib/labels";
import { useViewState } from "../../lib/viewState";
import { useCombinedDetail } from "./controller";
import { CombinedDialog } from "./CombinedDialog";
import { CombinedEntriesDialog } from "./CombinedEntriesDialog";
import { CombinedRulesDialog } from "./CombinedRulesDialog";

export function CombinedWorkspace({ meetId, combinedId }: { meetId: string; combinedId: string }) {
  const { go } = useNav();
  const { data: detail, error } = useCombinedDetail(combinedId, meetId);
  const [tab, setTab] = useViewState(`combined:tab:${combinedId}`, "components");
  const [editing, setEditing] = useState(false);
  const [entries, setEntries] = useState(false);
  const [rules, setRules] = useState(false);
  return <StPage flush toolbar={<StToolbar title={detail?.combined.name ?? "全能"} leading={<StIconButton label="返回全能列表" onClick={() => go({ page: "combineds", meetId })}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton>}>
    <StButton onClick={() => setEditing(true)} disabled={!detail}>编辑全能</StButton>
    <StButton onClick={() => setRules(true)} disabled={!detail}>计分规则</StButton>
  </StToolbar>}>
    {error && <StNotice tone="danger">{error}</StNotice>}
    {detail && <>
      <div className="subbar"><StSegmented value={tab} onChange={setTab} options={[{ value: "components", label: "分项" }, { value: "entries", label: "报名" }, { value: "results", label: "总成绩" }]} /><span className="subbar__spacer" /><StTag>{detail.complete ? "正式排名" : "暂列排名"}</StTag></div>
      {tab === "components" && <div className="table-card view-enter"><StTable<CombinedComponentView> fill scrollKey={`combined:components:${combinedId}`} rows={detail.components} rowKey={(c) => c.event.id} columns={[
        { key: "order", title: "顺序", width: "72px", render: (c) => c.ordinal },
        { key: "name", title: "分项", render: (c) => c.event.discipline },
        { key: "kind", title: "类型", render: (c) => kindLabels[c.event.kind] },
        { key: "actions", title: "", render: (c) => <span className="cell-actions"><StButton size="small" onClick={() => go({ page: "event", meetId, eventId: c.event.id, tab: "arrange" })}>编排</StButton><StButton size="small" onClick={() => go({ page: "event", meetId, eventId: c.event.id, tab: "results" })}>录入成绩</StButton></span> },
      ]} /></div>}
      {tab === "entries" && <div className="split__main view-enter"><StSection title={`${detail.athleteIds.length} 人报名`} padded={false} actions={<StButton variant="primary" onClick={() => setEntries(true)}>管理报名</StButton>}>
        <StTable<CombinedRow> rows={detail.rows} rowKey={(r) => r.athleteId} dense empty={<StEmpty>还没有报名</StEmpty>} columns={[{ key: "bib", title: "号码", render: (r) => r.bib }, { key: "name", title: "姓名", render: (r) => <>{r.name}{r.genderMismatch && <StTag tone="warning">性别不符</StTag>}</> }, { key: "unit", title: "单位", render: (r) => r.unitName }]} />
      </StSection></div>}
      {tab === "results" && <div className="table-card view-enter"><StTable<CombinedRow> fill scrollKey={`combined:results:${combinedId}`} rows={detail.rows} rowKey={(r) => r.athleteId} dense empty={<StEmpty>还没有报名</StEmpty>} columns={[
        { key: "rank", title: "名次", width: "56px", align: "end", render: (r) => r.excluded ? "—" : r.rank ?? "" },
        { key: "name", title: "姓名", width: "96px", render: (r) => r.name },
        { key: "total", title: "总分", width: "80px", align: "end", render: (r) => r.total },
        ...detail.components.map((c) => ({ key: c.event.id, title: c.event.discipline, width: "minmax(136px, 1fr)", render: (r: CombinedRow) => { const cell = r.cells.find((v) => v.eventId === c.event.id); return cell?.pending ? <StTag>待录</StTag> : <span>{cell?.markText} / {cell?.excluded ? "不列名次" : `${cell?.points ?? ""}分`}</span>; } })),
      ]} /></div>}
      {editing && <CombinedDialog meetId={meetId} detail={detail} onClose={() => setEditing(false)} />}
      {entries && <CombinedEntriesDialog detail={detail} onClose={() => setEntries(false)} />}
      {rules && <CombinedRulesDialog detail={detail} onClose={() => setRules(false)} />}
    </>}
  </StPage>;
}
