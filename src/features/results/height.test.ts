import { expect, test } from "bun:test";
import type { HeatView } from "../../mygo";
import { heightColumns, heightInputs, trialSymbol } from "./height";
import { sameRow, toRows } from "./rows";

const heat = { slots: [{ position: 1, competitorId: "a", athleteId: "a", result: { status: "OK", mark: 1500, markText: "1.50", rank: 1, qualify: "", attempts: [], note: "", bars: [{ height: 1500, heightText: "1.50", trials: "O" }] } }] } as HeatView;

test("外部刷新删列时保留有输入的高度，并按高度排序", () => {
  const rows = toRows(heat.slots, 0);
  rows[1]!.bars["1.55"] = "X";
  expect(heightColumns(heat, rows, ["1.45"], [])).toEqual(["1.45", "1.50", "1.55"]);
  expect(heightColumns(heat, rows, [], ["1.50"])).toEqual(["1.55"]);
});

test("新位置和换人先用读取数据，输入不会串给新运动员", () => {
  const rows = toRows(heat.slots, 0);
  const swapped = { ...heat, slots: [{ position: 1, competitorId: "b", athleteId: "b", result: null }, { position: 2, competitorId: "c", athleteId: "c", result: null }] };
  const inputs = heightInputs(swapped, rows, ["1.50"]);
  expect(inputs.map((row) => row.bars?.[0]?.trials)).toEqual(["", ""]);
});

test("空白试跳键与字段顺序不制造未保存修改", () => {
  const row = toRows(heat.slots, 0)[1]!;
  expect(sameRow(row, { ...row, bars: { "1.55": "", "1.50": "O" } })).toBe(true);
  expect(sameRow(row, { ...row, bars: { "1.50": "XO" } })).toBe(false);
});

test("单键记录试跳，其他键不记录", () => {
  expect(["o", "0", "x", "×", "-", "—"].map(trialSymbol)).toEqual(["O", "O", "X", "X", "-", "-"]);
  expect(trialSymbol("1")).toBeUndefined();
  expect(trialSymbol("Enter")).toBeUndefined();
});
