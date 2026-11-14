import type { EntrySummary, GroupEntries, UnitEntries } from "../../mygo";

/**
 * A node of the 报名 tree. group: "all", a group id, or "" for 未分组;
 * unit: "" for the whole group, else a unit under it. A meet without groups
 * has its units right under 全部 (group "all").
 */
export type RosterNode = { group: string; unit: string };

export const allNode: RosterNode = { group: "all", unit: "" };

/** The view-state key of the tree's node; other pages set it to open a group or unit on 报名. */
export const rosterNodeKey = (meetId: string) => `entries:node:${meetId}`;
/** The view-state key of whether a narrow window shows the athletes rather than the tree. */
export const rosterDetailKey = (meetId: string) => `entries:detail:${meetId}`;

export const nodeKey = (n: RosterNode) => `${n.group}|${n.unit}`;

export function parseNode(key: string): RosterNode {
  const i = key.indexOf("|");
  return i < 0 ? allNode : { group: key.slice(0, i), unit: key.slice(i + 1) };
}

export type TreeRow =
  | { kind: "all"; node: RosterNode }
  | { kind: "group"; node: RosterNode; group: GroupEntries }
  | { kind: "unit"; node: RosterNode; unit: UnitEntries; athletes: number; indent: boolean };

/** The tree's rows from top to bottom: 全部, then each group and its units, or the units alone. */
export function treeRows(s: EntrySummary): TreeRow[] {
  const rows: TreeRow[] = [{ kind: "all", node: allNode }];
  if (s.groups.length === 0) {
    for (const u of s.units) rows.push({ kind: "unit", node: { group: "all", unit: u.unit.id }, unit: u, athletes: u.athletes, indent: false });
    return rows;
  }
  const units = new Map(s.units.map((u) => [u.unit.id, u]));
  for (const g of s.groups) {
    rows.push({ kind: "group", node: { group: g.id, unit: "" }, group: g });
    for (const m of g.members ?? []) {
      const u = units.get(m.unitId);
      if (u) rows.push({ kind: "unit", node: { group: g.id, unit: m.unitId }, unit: u, athletes: m.athletes, indent: true });
    }
  }
  return rows;
}

/** The node if the tree still has it; a deleted group or unit, or a unit moved out of its group, reads as 全部. */
export function knownNode(rows: TreeRow[], node: RosterNode): RosterNode {
  const key = nodeKey(node);
  return rows.some((r) => nodeKey(r.node) === key) ? node : allNode;
}

/** Whether an athlete is under the node: in its group and its unit. */
export function inNode(node: RosterNode, a: { unitId: string; groupId?: string }) {
  return (node.group === "all" || (a.groupId ?? "") === node.group) && (node.unit === "" || a.unitId === node.unit);
}

/** The node's group and unit names, outermost first. */
export function nodePath(s: EntrySummary, node: RosterNode): string[] {
  const group = node.group === "all" ? undefined : s.groups.find((g) => g.id === node.group);
  const unit = node.unit ? s.units.find((u) => u.unit.id === node.unit) : undefined;
  return [group?.name, unit?.unit.name].filter((v): v is string => !!v);
}

export type ExportChoice = { label: string; ids: string[]; byGroup: boolean };

/**
 * What 「导出登记表」 offers for the node: one form per unit under it, or one
 * per group. 全部 exports every unit (or every group); a unit just its own.
 * Under 未分组 only the units of no group have a form of their own there.
 */
export function exportChoices(s: EntrySummary, node: RosterNode): ExportChoice[] {
  if (s.units.length === 0) return [];
  if (node.unit) {
    const u = s.units.find((r) => r.unit.id === node.unit);
    return u ? [{ label: `「${u.unit.name}」`, ids: [u.unit.id], byGroup: false }] : [];
  }
  if (node.group === "all") {
    const groups = s.groups.filter((g) => g.units > 0).length;
    return [
      { label: `每个单位一份（${s.units.length} 份）`, ids: [], byGroup: false },
      ...(groups > 0 ? [{ label: `每个组别一份（${groups} 份）`, ids: [], byGroup: true }] : []),
    ];
  }
  const g = s.groups.find((r) => r.id === node.group);
  if (!g || g.units === 0) return [];
  const own = new Set(s.units.filter((u) => (g.id === "" ? !u.unit.groupIds?.length : u.unit.groupIds?.includes(g.id))).map((u) => u.unit.id));
  const ids = (g.members ?? []).map((m) => m.unitId).filter((id) => own.has(id));
  return [
    { label: `每个单位一份（${ids.length} 份）`, ids, byGroup: false },
    { label: `「${g.name}」一份`, ids: [g.id], byGroup: true },
  ];
}
