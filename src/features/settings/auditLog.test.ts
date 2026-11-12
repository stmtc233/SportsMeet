import { describe, expect, test } from "bun:test";
import type { AuditRow } from "../../mygo";
import { appendOlder, mergeNewest } from "./auditLog";

const row = (id: number, at = id * 1000): AuditRow => ({ id, at, atText: "", source: "ui", sourceText: "界面", action: "unit.created", actionText: "新建参赛单位", meetId: "", meetName: "", eventId: "", heatId: "", message: String(id) });
const ids = (rows: AuditRow[]) => rows.map((r) => r.id);
const cursor = (r: AuditRow) => ({ at: r.at, id: r.id });

describe("operation log pages", () => {
  test("the first page is taken as it is", () => {
    expect(mergeNewest(null, { rows: [row(3), row(2)], next: cursor(row(2)) })).toEqual({ rows: [row(3), row(2)], next: cursor(row(2)) });
  });

  test("older pages append, and a reload keeps them", () => {
    let log = mergeNewest(null, { rows: [row(6), row(5)], next: cursor(row(5)) });
    log = appendOlder(log, { rows: [row(4), row(3)], next: cursor(row(3)) });
    // A new row pushes 5 off the newest page; it stays loaded.
    log = mergeNewest(log, { rows: [row(7), row(6)], next: cursor(row(6)) });
    expect(ids(log.rows)).toEqual([7, 6, 5, 4, 3]);
    expect(log.next).toEqual(cursor(row(3)));
  });

  test("a page that no longer reaches the loaded rows starts over", () => {
    let log = mergeNewest(null, { rows: [row(2), row(1)], next: null });
    log = mergeNewest(log, { rows: [row(9), row(8)], next: cursor(row(8)) });
    expect(ids(log.rows)).toEqual([9, 8]);
    expect(log.next).toEqual(cursor(row(8)));
  });

  test("equal times keep insertion order, newest first", () => {
    const log = mergeNewest(mergeNewest(null, { rows: [row(1, 5)], next: null }), { rows: [row(3, 5), row(2, 5), row(1, 5)], next: null });
    expect(ids(log.rows)).toEqual([3, 2, 1]);
    expect(log.next).toBeNull();
  });
});
