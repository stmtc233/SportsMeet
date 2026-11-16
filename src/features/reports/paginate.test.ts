import { describe, expect, test } from "bun:test";
import { a4, continuedTitle, fitScale, headTexts, pageLabel, readPaper, sheetRule, zoomScale } from "./paginate";

describe("fitScale", () => {
  test("fits the page between the gutters", () => {
    expect(fitScale(826, 794)).toBe(1);
    expect(fitScale(429, 794)).toBe(0.5);
  });
  test("a pane too narrow keeps the smallest scale", () => {
    expect(fitScale(0, 794)).toBe(0.2);
  });
  test("a page not measured yet", () => {
    expect(fitScale(800, 0)).toBe(1);
  });
});

test("zoomScale: fixed choices ignore the pane", () => {
  expect(zoomScale("0.75", 300, 794)).toBe(0.75);
  expect(zoomScale("fit", 429, 794)).toBe(0.5);
});

// Just what continuedTitle reads of an element.
type Node = { tagName: string; children: Node[]; childNodes: { nodeType: number; textContent: string }[]; classList: { contains: (c: string) => boolean } };
const el = (tagName: string, children: Node[] = [], text = "", classes: string[] = []): Node => ({
  tagName, children, childNodes: text ? [{ nodeType: 3, textContent: text }, { nodeType: 1, textContent: "small" }] : [],
  classList: { contains: (c: string) => classes.includes(c) },
});
const at = (...open: Node[]) => continuedTitle(open as unknown as Element[]);

describe("continuedTitle", () => {
  test("the event and the round a table goes on in", () => {
    const table = el("TABLE");
    const round = el("DIV", [el("H3", [], "决赛"), table]);
    const event = el("SECTION", [el("H2", [], "男子100m 初一组"), el("DIV", [el("H3", [], "预赛")]), round]);
    expect(at(event, round, table)).toBe("男子100m 初一组 · 决赛");
  });
  test("each level's last heading before the break", () => {
    const table = el("TABLE");
    const schedule = el("SECTION", [el("H2", [], "竞赛日程表"), el("H3", [], "2026-11-12 周四"), el("TABLE"), el("H3", [], "2026-11-13 周五"), table]);
    expect(at(schedule, table)).toBe("竞赛日程表 · 2026-11-13 周五");
  });
  test("the group title is left out; nothing titled is empty", () => {
    const group = el("SECTION", [el("H2", [], "初一组", ["report-group__title"])]);
    expect(at(group)).toBe("");
    expect(at(el("DIV"))).toBe("");
  });
});

describe("readPaper", () => {
  test("A4 and the 15mm margins when the report does not say", () => {
    expect(readPaper({})).toEqual(a4);
    expect(readPaper({ paper: "abc", margin: "1,2" })).toEqual(a4);
    expect(readPaper({ paper: "0x297", margin: "-1,2,3,4" })).toEqual(a4);
  });
  test("a custom paper and margins", () => {
    expect(readPaper({ paper: "195x270", margin: "20,15,25,15" })).toEqual({ width: 195, height: 270, margin: [20, 15, 25, 15] });
    expect(readPaper({ paper: "215.9x279.4", margin: "10.5,10.5,10.5,10.5" })).toEqual({ width: 215.9, height: 279.4, margin: [10.5, 10.5, 10.5, 10.5] });
  });
  test("landscape arrives as a wide paper", () => {
    const paper = readPaper({ paper: "420x297", margin: "15,15,15,15" });
    expect(paper.width).toBe(420);
    expect(paper.height).toBe(297);
    expect(sheetRule(paper)).toBe("@page { size: 420mm 297mm; margin: 0; }");
  });
  test("the page rule of A4", () => {
    expect(sheetRule(a4)).toBe("@page { size: 210mm 297mm; margin: 0; }");
  });
});

describe("pageLabel", () => {
  test("第 N 页 / 共 M 页 is the default", () => {
    expect(pageLabel(2, 9)).toBe("第 2 页 / 共 9 页");
    expect(pageLabel(2, 9, "n_of_m")).toBe("第 2 页 / 共 9 页");
    expect(pageLabel(2, 9, "unknown")).toBe("第 2 页 / 共 9 页");
  });
  test("dash and none", () => {
    expect(pageLabel(3, 9, "dash")).toBe("- 3 -");
    expect(pageLabel(3, 9, "none")).toBe("");
  });
  test("the report's own foot text wins", () => {
    expect(pageLabel(3, 9, "dash", "校运会 {page}/{pages}")).toBe("校运会 3/9");
  });
});

describe("headTexts", () => {
  test("no running head without data-meet", () => {
    expect(headTexts({})).toBeNull();
    expect(headTexts({ title: "成绩册" })).toBeNull();
  });
  test("the meet and the report, or the report's own texts", () => {
    expect(headTexts({ meet: "校运会", title: "成绩册" })).toEqual(["校运会", "成绩册"]);
    expect(headTexts({ meet: "校运会", title: "成绩册", headLeft: "内部资料", headRight: "第一届" })).toEqual(["内部资料", "第一届"]);
    expect(headTexts({ meet: "校运会", headRight: "" })).toEqual(["校运会", ""]);
  });
});
