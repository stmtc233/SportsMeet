// The operation log as the settings page holds it: the newest page, read
// again on every data change, merged with the older pages already loaded.
// Log rows never change, so merging is a union by id.
import type { AuditCursor, AuditPage, AuditRow } from "../../mygo";

export interface AuditLog {
  rows: AuditRow[];
  /** Where to continue for older rows; null when everything is loaded. */
  next: AuditCursor | null;
}

/** Newest first: time, then id (insertion order). */
const newer = (a: { at: number; id: number }, b: { at: number; id: number }) => b.at - a.at || b.id - a.id;

function union(a: AuditRow[], b: AuditRow[]): AuditRow[] {
  const byId = new Map<number, AuditRow>();
  for (const row of [...a, ...b]) byId.set(row.id, row);
  return [...byId.values()].sort(newer);
}

/**
 * Merges a fresh newest page into what is loaded. When more rows arrived
 * than a page holds, the page no longer reaches what was loaded and the
 * log starts again from that page, so no gap is shown.
 */
export function mergeNewest(loaded: AuditLog | null, newest: AuditPage): AuditLog {
  if (!loaded || loaded.rows.length === 0) return { rows: newest.rows, next: newest.next };
  const last = newest.rows[newest.rows.length - 1];
  const reaches = newest.next === null || last === undefined || newer(last, loaded.rows[0]!) >= 0;
  if (!reaches) return { rows: newest.rows, next: newest.next };
  return { rows: union(newest.rows, loaded.rows), next: newest.next === null ? null : loaded.next };
}

/** Appends an older page read from loaded.next. */
export function appendOlder(loaded: AuditLog, older: AuditPage): AuditLog {
  return { rows: union(loaded.rows, older.rows), next: older.next };
}
