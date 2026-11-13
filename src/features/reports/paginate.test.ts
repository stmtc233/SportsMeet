import { describe, expect, test } from "bun:test";
import { continuedTitle, fitScale, zoomScale } from "./paginate";

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
