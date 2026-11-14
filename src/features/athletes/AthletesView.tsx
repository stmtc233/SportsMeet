import { useMemo, useState } from "react";
import { useViewState } from "../../lib/viewState";
import { IconAdd, IconBack, StButton, StEmpty, StIconButton, StSearchField, StTable, StTag, iconStroke } from "../../design";
import type { Athlete, AthleteRow } from "../../mygo";
import { inNode, type RosterNode } from "../entries/tree";
import { GroupsController, useGroups } from "../groups/controller";
import { GroupMenu, ungrouped } from "../groups/GroupFields";
import { DeleteDialog } from "../meets/DeleteDialog";
import { useUnits } from "../units/controller";
import { AthleteDialog } from "./AthleteDialog";
import { AthletesController, useAthletes } from "./controller";

/**
 * The right of the 报名 page: the athletes under the tree's node, found by
 * name or number. path names the node (group, then unit); onBack returns to
 * the tree in a narrow window.
 */
export function AthletesView({ meetId, node, path, onBack }: { meetId: string; node: RosterNode; path: string[]; onBack: () => void }) {
  const { data: rows } = useAthletes(meetId);
  const { data: units } = useUnits(meetId);
  const { data: groups } = useGroups(meetId);
  const [query, setQuery] = useViewState(`athletes:query:${meetId}`, "");
  const [selected, setSelected] = useState(new Set<string>());
  const [editing, setEditing] = useState<{ athlete?: Athlete } | null>(null);
  const [deleting, setDeleting] = useState(false);

  const under = useMemo(() => (rows ?? []).filter((r) => inNode(node, r.athlete)), [rows, node]);
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q === "" ? under : under.filter((r) => [r.athlete.name, r.athlete.bib, r.athlete.studentNumber, r.athlete.className].some((v) => v.toLowerCase().includes(q)));
  }, [under, query]);
  const picked = shown.filter((r) => selected.has(r.athlete.id));
  // A new athlete starts in the node's unit and group.
  const unit = units?.find((u) => u.id === node.unit);
  const startGroup = node.group !== "all" ? node.group : unit?.groupIds?.length === 1 ? unit.groupIds[0]! : "";
  const name = path.at(-1) ?? "全部";
  const outer = path.slice(0, -1);

  return (
    <section className="roster-main" aria-label={path.join(" ") || "全部运动员"}>
      <div className="filters roster-main__bar">
        <span className="roster-main__back">
          <StIconButton label="组别与单位" onClick={onBack}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton>
        </span>
        <h2 className="roster-main__title">
          {outer.map((p) => <span key={p} className="roster-main__outer">{p}</span>)}
          {name}
          <span className="roster-main__count">{rows ? `${under.length} 人` : ""}</span>
        </h2>
        <span className="filters__spacer" />
        <StSearchField value={query} onChange={setQuery} placeholder="搜索姓名、号码、学号" />
        <GroupMenu meetId={meetId} count={picked.length}
          onPick={(id) => GroupsController.assign(meetId, picked.map((r) => ({ kind: "athlete", id: r.athlete.id, groupIds: id ? [id] : [] })))} />
        <StButton variant="danger" disabled={picked.length === 0 || picked.some((r) => r.teamMember || r.placed)} onClick={() => setDeleting(true)} reserveWidthFor={["删除 999 人"]}>
          {picked.length > 0 ? `删除 ${picked.length} 人` : "删除"}
        </StButton>
        <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加运动员</StButton>
      </div>
      <div className="table-card">
        <StTable<AthleteRow> fill rows={shown} loading={!rows} scrollKey={`athletes:${meetId}`} rowKey={(r) => r.athlete.id} rowLabel={(r) => r.athlete.name} actionColumn="name" selected={selected} onSelect={setSelected} onRowClick={(r) => setEditing({ athlete: r.athlete })}
          empty={<StEmpty>{rows?.length === 0 ? "还没有运动员" : under.length === 0 ? "这里还没有运动员" : "没有符合条件的运动员"}</StEmpty>}
          columns={[
            { key: "bib", title: "号码", width: "88px", render: (r) => r.athlete.bib },
            { key: "name", title: "姓名", width: "minmax(120px, 1.2fr)", render: (r) => <span className="cell-strong">{r.athlete.name}</span> },
            { key: "gender", title: "性别", width: "64px", render: (r) => r.athlete.gender },
            // The node already says the unit and the group.
            ...(node.unit ? [] : [{ key: "unit", title: "参赛单位", width: "minmax(120px, 1.5fr)", render: (r: AthleteRow) => r.unitName }]),
            ...(groups?.length && node.group === "all" ? [{ key: "group", title: "组别", width: "minmax(72px, 0.8fr)", render: (r: AthleteRow) => groups.find((g) => g.id === r.athlete.groupId)?.name ?? ungrouped }] : []),
            { key: "class", title: "班级", width: "minmax(96px, 1fr)", render: (r) => [r.athlete.grade, r.athlete.className].filter(Boolean).join(" ") },
            { key: "number", title: "学号", width: "minmax(96px, 1fr)", render: (r) => r.athlete.studentNumber },
            { key: "entries", title: "报名", width: "88px", align: "end", render: (r) => (r.entries > 0 ? <StTag tone={r.overLimit ? "warning" : r.placed ? "info" : "neutral"}>{r.entries} 项</StTag> : "") },
          ]} />
      </div>
      <AthleteDialog meetId={meetId} open={editing !== null} athlete={editing?.athlete} units={units ?? []} unitId={node.unit} groupId={startGroup} onClose={() => setEditing(null)} />
      <DeleteDialog open={deleting} title={`删除 ${picked.length} 名运动员？`} onClose={() => setDeleting(false)}
        onDelete={async () => {
          const ok = await AthletesController.remove(meetId, picked.map((r) => r.athlete.id));
          if (ok) setSelected(new Set());
          return ok;
        }}>
        他们的报名会一并删除，无法撤销。
      </DeleteDialog>
    </section>
  );
}
