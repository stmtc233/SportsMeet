import { describe, expect, test } from "bun:test";
import { fitScale, zoomScale } from "./paginate";

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
