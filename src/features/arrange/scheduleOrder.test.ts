import { expect, test } from "bun:test";
import { moveItem, toggleItem } from "./scheduleOrder";

test("勾选追加到末尾，再勾去掉", () => {
  expect(toggleItem(["a"], "b")).toEqual(["a", "b"]);
  expect(toggleItem(["a", "b", "c"], "b")).toEqual(["a", "c"]);
});

test("上移下移只换相邻，两端不动", () => {
  expect(moveItem(["a", "b", "c"], "c", -1)).toEqual(["a", "c", "b"]);
  expect(moveItem(["a", "b", "c"], "a", 1)).toEqual(["b", "a", "c"]);
  const order = ["a", "b"];
  expect(moveItem(order, "a", -1)).toBe(order);
  expect(moveItem(order, "b", 1)).toBe(order);
  expect(moveItem(order, "x", 1)).toBe(order);
});
