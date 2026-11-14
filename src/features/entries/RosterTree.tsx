import { useRef, useState, type ReactNode } from "react";
import { IconAdd, IconMore, StButton, StIconButton, StListItem, StMenu, StNavGroup, StTag, iconStroke, type StMenuItem } from "../../design";
import { useScrollMemory } from "../../lib/viewState";
import type { GroupView, Unit, UnitEntries } from "../../mygo";
import { GroupsController, useGroupSuggestions, useGroups } from "../groups/controller";
import { GroupDeleteDialog, GroupDialog } from "../groups/GroupDialog";
import { GroupOrganizeDialog } from "../groups/GroupOrganizeDialog";
import { DeleteDialog } from "../meets/DeleteDialog";
import { UnitsController } from "../units/controller";
import { UnitDialog } from "../units/UnitDialog";
import { useEntrySummary } from "./controller";
import { nodeKey, treeRows, type RosterNode, type TreeRow } from "./tree";

/**
 * The left of the 报名 page: 全部 → 组别 → 参赛单位, each with its athletes
 * and the units' 报名进度. Groups and units are added, edited, ordered and
 * deleted here, where they are used.
 */
export function RosterTree({ meetId, node, total, onSelect }: {
  meetId: string;
  node: RosterNode;
  /** All the meet's athletes; undefined while reading. */
  total: number | undefined;
  onSelect: (node: RosterNode) => void;
}) {
  const { data: summary } = useEntrySummary(meetId);
  const { data: groups } = useGroups(meetId);
  const { data: suggestions } = useGroupSuggestions(meetId);
  const [editingGroup, setEditingGroup] = useState<{ group?: GroupView } | null>(null);
  const [deletingGroup, setDeletingGroup] = useState<GroupView | null>(null);
  const [editingUnit, setEditingUnit] = useState<{ unit?: Unit } | null>(null);
  const [deletingUnit, setDeletingUnit] = useState<Unit | null>(null);
  const [organizing, setOrganizing] = useState(false);
  const [moving, setMoving] = useState(false);
  const list = useRef<HTMLDivElement>(null);
  useScrollMemory(list, `entries:tree:${meetId}`);

  const rows = summary ? treeRows(summary) : [];
  const grouped = (groups?.length ?? 0) > 0;
  const selected = nodeKey(node);
  // A unit added with a group selected goes into it.
  const startGroup = node.group !== "all" && node.group !== "" ? node.group : undefined;

  const move = async (id: string, by: number) => {
    if (!groups) return;
    const ids = groups.map((g) => g.id);
    const i = ids.indexOf(id);
    [ids[i], ids[i + by]] = [ids[i + by]!, ids[i]!];
    setMoving(true);
    await GroupsController.reorder(meetId, ids);
    setMoving(false);
  };
  const groupMenu = (id: string): StMenuItem[] => {
    const g = groups?.find((r) => r.id === id);
    if (!g || !groups) return [];
    const i = groups.indexOf(g);
    return [
      { label: "编辑组别", onSelect: () => setEditingGroup({ group: g }) },
      { label: "上移", disabled: moving || i === 0, onSelect: () => void move(id, -1) },
      { label: "下移", disabled: moving || i === groups.length - 1, onSelect: () => void move(id, 1) },
      { label: "删除组别", danger: true, separated: true, onSelect: () => setDeletingGroup(g) },
    ];
  };
  const unitMenu = (u: Unit): StMenuItem[] => [
    { label: "编辑单位", onSelect: () => setEditingUnit({ unit: u }) },
    { label: "删除单位", danger: true, separated: true, onSelect: () => setDeletingUnit(u) },
  ];

  const item = (r: TreeRow) => {
    const props = { selected: nodeKey(r.node) === selected, onClick: () => onSelect(r.node) };
    switch (r.kind) {
      case "all":
        return <StListItem key="all" title="全部" trailing={<Count n={total} />} {...props} />;
      case "group":
        return <StListItem key={nodeKey(r.node)} title={<span className="roster-tree__group">{r.group.name}</span>} {...props}
          trailing={<><Count n={r.group.athletes} />{r.group.id && <More label={`${r.group.name} 更多`} items={groupMenu(r.group.id)} />}</>} />;
      default:
        return <StListItem key={nodeKey(r.node)} title={r.unit.unit.name} indent={r.indent} {...props}
          trailing={<>{status(r.unit)}<Count n={r.athletes} /><More label={`${r.unit.unit.name} 更多`} items={unitMenu(r.unit.unit)} /></>} />;
    }
  };

  return (
    <aside className="split__side roster-tree" aria-label="组别与单位">
      <div className="split__side-head">
        <span>{grouped ? "组别与单位" : "参赛单位"}</span>
        {grouped && !!suggestions?.length && <StButton size="small" variant="ghost" onClick={() => setOrganizing(true)}>整理归属</StButton>}
      </div>
      <div ref={list} className="split__side-list">
        <StNavGroup>{rows.map(item)}</StNavGroup>
      </div>
      <div className="split__side-foot roster-tree__foot">
        <StButton size="small" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditingGroup({})}>添加组别</StButton>
        <StButton size="small" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditingUnit({})}>添加单位</StButton>
      </div>
      {editingGroup && <GroupDialog key={editingGroup.group?.id ?? "new"} meetId={meetId} group={editingGroup.group} onClose={() => setEditingGroup(null)} />}
      <GroupDeleteDialog meetId={meetId} group={deletingGroup} onClose={() => setDeletingGroup(null)} />
      {organizing && <GroupOrganizeDialog meetId={meetId} onClose={() => setOrganizing(false)} />}
      <UnitDialog meetId={meetId} open={editingUnit !== null} unit={editingUnit?.unit} groupId={startGroup} onClose={() => setEditingUnit(null)} />
      <DeleteDialog open={deletingUnit !== null} title={`删除「${deletingUnit?.name ?? ""}」？`} onClose={() => setDeletingUnit(null)}
        onDelete={() => UnitsController.remove(meetId, deletingUnit!.id)}>
        该单位的运动员会保留，但不再属于任何单位。
      </DeleteDialog>
    </aside>
  );
}

/** 未报名 for a unit with neither entries nor teams, 超限 N when over a limit. */
function status(u: UnitEntries): ReactNode {
  if (u.entries === 0 && u.teams === 0) return <StTag>未报名</StTag>;
  if (u.over > 0) return <StTag tone="warning">超限 {u.over}</StTag>;
  return null;
}

function Count({ n }: { n: number | undefined }) {
  return <span className="roster-tree__count">{n ?? ""}</span>;
}

/** A row's own menu; opening it leaves the row's selection alone. */
function More({ label, items }: { label: string; items: StMenuItem[] }) {
  return (
    <span className="roster-tree__more" onClick={(e) => e.stopPropagation()}>
      <StMenu items={items} trigger={(p) => (
        <StIconButton size="small" label={label} {...p}>
          <IconMore size={16} strokeWidth={iconStroke} />
        </StIconButton>
      )} />
    </span>
  );
}
