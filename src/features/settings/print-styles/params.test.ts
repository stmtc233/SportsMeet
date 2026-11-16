import { describe, expect, test } from "bun:test";
import type { Params, PrintParamRules } from "../../../mygo";
import rulesJson from "../../../../testdata/print/param-rules.json";
import { firstProblem, formatNumber, marginsTooBig, numberError, paperMM, paperPatch, parseNumber } from "./params";

// The limits as Go gives them; internal/service print_rules_test.go keeps the file current.
const rules = rulesJson as PrintParamRules;

const a4: Params = {
  paper: "a4", paperWidth: 0, paperHeight: 0, orientation: "portrait",
  marginTop: 15, marginRight: 15, marginBottom: 15, marginLeft: 15,
  font: "sans", fontSize: 9.5, lineHeight: 1.4, titleSize: 18, eventTitleSize: 12.5,
  tableLines: "rules", lineWidth: 1, header: true, pageNumber: "n_of_m", rosterColumns: 4,
  showUnit: true, showSeed: true, showBib: true,
};

describe("parseNumber", () => {
  test("reads plain numbers", () => {
    expect(parseNumber("12")).toBe(12);
    expect(parseNumber(" 9.5 ")).toBe(9.5);
    expect(parseNumber(".5")).toBe(0.5);
    expect(parseNumber("9.")).toBe(9);
  });
  test("rejects text, empty and trailing junk", () => {
    for (const t of ["", "abc", ".", "1e3", "1,5", "--1", "Infinity"]) expect(parseNumber(t)).toBeUndefined();
  });
  test("formatNumber drops trailing zeros", () => {
    expect(formatNumber(1.0)).toBe("1");
    expect(formatNumber(215.9)).toBe("215.9");
  });
});

describe("numberError", () => {
  test("a value in range is good", () => {
    expect(numberError(rules, "fontSize", "9.5")).toBe("");
    expect(numberError(rules, "marginTop", "0")).toBe("");
    expect(numberError(rules, "rosterColumns", "8")).toBe("");
  });
  test("a value out of range names the range", () => {
    expect(numberError(rules, "fontSize", "5")).toBe("正文字号需在 6–24 pt 之间");
    expect(numberError(rules, "lineHeight", "3")).toBe("行距需在 1–2.5 倍之间");
    expect(numberError(rules, "lineWidth", "0")).toBe("线宽需在 0.1–3 pt 之间");
    expect(numberError(rules, "paperWidth", "99")).toBe("纸张宽需在 100–1000 mm 之间");
  });
  test("an integer field refuses fractions", () => {
    expect(numberError(rules, "rosterColumns", "2.5")).toBe("请输入整数");
    expect(numberError(rules, "rosterColumns", "9")).toBe("名单每行人数需在 1–8 人之间");
  });
  test("text that is no number", () => {
    expect(numberError(rules, "fontSize", "")).toBe("请输入数字");
  });
});

describe("paper and margins", () => {
  test("orientation swaps the sheet", () => {
    expect(paperMM(rules, a4)).toEqual([210, 297]);
    expect(paperMM(rules, { ...a4, orientation: "landscape" })).toEqual([297, 210]);
    expect(paperMM(rules, { ...a4, paper: "custom", paperWidth: 120, paperHeight: 150 })).toEqual([120, 150]);
  });
  test("margins must leave 50 mm to print in", () => {
    expect(marginsTooBig(rules, a4)).toBe(false);
    expect(marginsTooBig(rules, { ...a4, marginLeft: 60, marginRight: 60, paper: "custom", paperWidth: 160, paperHeight: 200 })).toBe(true);
  });
  test("firstProblem finds the first wrong value", () => {
    expect(firstProblem(rules, a4)).toBe("");
    expect(firstProblem(rules, { ...a4, titleSize: 4 })).toBe("一级标题字号需在 8–48 pt 之间");
    // A standard paper ignores a stale custom size.
    expect(firstProblem(rules, { ...a4, paperWidth: 1 })).toBe("");
    expect(firstProblem(rules, { ...a4, paper: "custom", paperWidth: 1, paperHeight: 200 })).toBe("纸张宽需在 100–1000 mm 之间");
    expect(firstProblem(rules, { ...a4, paper: "custom", paperWidth: 100, paperHeight: 100, marginLeft: 30, marginRight: 30 })).toBe("边距太大，纸上放不下内容");
  });
  test("a custom sheet starts from the one it replaces", () => {
    expect(paperPatch(rules, a4, "custom")).toEqual({ paper: "custom", paperWidth: 210, paperHeight: 297 });
    expect(paperPatch(rules, { ...a4, paper: "custom", paperWidth: 120, paperHeight: 150 }, "custom")).toEqual({ paper: "custom" });
    expect(paperPatch(rules, a4, "a3")).toEqual({ paper: "a3" });
  });
});
