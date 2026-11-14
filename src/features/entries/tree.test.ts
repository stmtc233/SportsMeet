import { expect, test } from "bun:test";
import type { EntrySummary, UnitEntries } from "../../mygo";
import { allNode, exportChoices, inNode, knownNode, nodeKey, nodePath, parseNode, treeRows } from "./tree";

const unit = (id: string, name: string, groupIds: string[], athletes = 2): UnitEntries =>
  ({ unit: { id, name, groupIds, meetId: "m", shortName: "" }, athletes, entries: 0, teams: 0, over: 0 });

const flat: EntrySummary = { units: [unit("u1", "一班", []), unit("u2", "二班", [])], groups: [], limited: false };

// 一班 in 高一 with one athlete of no group; 二班 in no group.
const grouped: EntrySummary = {
  units: [unit("u1", "一班", ["g1"]), unit("u2", "二班", [])],
  groups: [
    { id: "g1", name: "高一", units: 1, athletes: 1, entries: 0, members: [{ unitId: "u1", athletes: 1 }] },
    { id: "", name: "未分组", units: 1, athletes: 3, entries: 0, members: [{ unitId: "u1", athletes: 1 }, { unitId: "u2", athletes: 2 }] },
  ],
  limited: false,
};

test("a meet without groups has its units under 全部", () => {
  const rows = treeRows(flat);
  expect(rows.map((r) => r.kind)).toEqual(["all", "unit", "unit"]);
  expect(rows[1]).toMatchObject({ node: { group: "all", unit: "u1" }, athletes: 2, indent: false });
});

test("groups list their units with the athletes of that group", () => {
  const rows = treeRows(grouped);
  expect(rows.map((r) => nodeKey(r.node))).toEqual(["all|", "g1|", "g1|u1", "|", "|u1", "|u2"]);
  expect(rows[2]).toMatchObject({ athletes: 1, indent: true });
  expect(rows[4]).toMatchObject({ athletes: 1 });
});

test("node keys round-trip and unknown nodes read as 全部", () => {
  expect(parseNode(nodeKey({ group: "", unit: "u2" }))).toEqual({ group: "", unit: "u2" });
  expect(parseNode("junk")).toEqual(allNode);
  const rows = treeRows(grouped);
  expect(knownNode(rows, { group: "g1", unit: "u1" })).toEqual({ group: "g1", unit: "u1" });
  expect(knownNode(rows, { group: "g1", unit: "u2" })).toEqual(allNode);
  expect(knownNode(rows, { group: "gone", unit: "" })).toEqual(allNode);
});

test("an athlete is under a node by group and unit", () => {
  const a = { unitId: "u1", groupId: "" };
  expect(inNode(allNode, a)).toBe(true);
  expect(inNode({ group: "g1", unit: "u1" }, a)).toBe(false);
  expect(inNode({ group: "", unit: "u1" }, a)).toBe(true);
  expect(inNode({ group: "all", unit: "u2" }, a)).toBe(false);
  expect(nodePath(grouped, { group: "g1", unit: "u1" })).toEqual(["高一", "一班"]);
  expect(nodePath(grouped, allNode)).toEqual([]);
});

test("export offers the node's units or groups", () => {
  expect(exportChoices(flat, allNode)).toEqual([{ label: "每个单位一份（2 份）", ids: [], byGroup: false }]);
  expect(exportChoices(grouped, allNode).map((c) => c.label)).toEqual(["每个单位一份（2 份）", "每个组别一份（2 份）"]);
  expect(exportChoices(grouped, { group: "g1", unit: "u1" })).toEqual([{ label: "「一班」", ids: ["u1"], byGroup: false }]);
  expect(exportChoices(grouped, { group: "g1", unit: "" })).toEqual([
    { label: "每个单位一份（1 份）", ids: ["u1"], byGroup: false },
    { label: "「高一」一份", ids: ["g1"], byGroup: true },
  ]);
  // 一班 is under 未分组 for its loose athlete only; its form belongs to 高一.
  expect(exportChoices(grouped, { group: "", unit: "" })[0]).toEqual({ label: "每个单位一份（1 份）", ids: ["u2"], byGroup: false });
  expect(exportChoices({ ...flat, units: [] }, allNode)).toEqual([]);
});
