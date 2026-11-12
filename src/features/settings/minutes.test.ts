import { describe, expect, test } from "bun:test";
import { minuteOptions } from "./minutes";

describe("minuteOptions", () => {
  test("zero has its own label", () => {
    expect(minuteOptions([0, 20], 20, "不显示")).toEqual([{ value: "0", label: "不显示" }, { value: "20", label: "20 分钟" }]);
  });
  test("a stored value outside the list stays", () => {
    expect(minuteOptions([0, 20], 25, "不显示").map((o) => o.value)).toEqual(["0", "20", "25"]);
  });
});
