import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { useScrollMemory } from "../lib/viewState";
import { StCheckbox } from "./StFields";
import { StEmpty } from "./StLayout";

export interface StColumn<T> {
  key: string;
  title: ReactNode;
  render: (row: T, index: number) => ReactNode;
  /** CSS width (a track of the grid): "120px", "1fr", "minmax(120px, 2fr)". */
  width?: string;
  /** Numbers align right. */
  align?: "start" | "end" | "center";
}

interface StTableProps<T> {
  columns: StColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  /** Selected row keys; passing it shows the check column. */
  selected?: Set<string>;
  onSelect?: (keys: Set<string>) => void;
  /** Shown when there are no rows; plain text is laid out as StEmpty. */
  empty?: ReactNode;
  /** Fills its container and scrolls inside, the header staying on top. */
  fill?: boolean;
  dense?: boolean;
  /** Rows still loading: the empty placeholder waits, so it does not flash. */
  loading?: boolean;
  /** With fill: the scroll position comes back under this key. */
  scrollKey?: string;
}

const rowHeight = { normal: 40, dense: 34 };
const overscan = 12;

/**
 * A data table on a CSS grid. The header sticks to the top; long lists
 * render only the rows in view (rows have a fixed height).
 */
export function StTable<T>({ columns, rows, rowKey, onRowClick, selected, onSelect, empty, fill, dense, loading, scrollKey }: StTableProps<T>) {
  const scroller = useRef<HTMLDivElement>(null);
  useScrollMemory(scroller, fill ? scrollKey : undefined);
  const [top, setTop] = useState(0);
  const [view, setView] = useState(800);
  const h = dense ? rowHeight.dense : rowHeight.normal;
  const selectable = selected !== undefined && onSelect !== undefined;
  const template = [selectable ? "40px" : "", ...columns.map((c) => c.width ?? "1fr")].filter(Boolean).join(" ");

  const virtual = fill && rows.length > 200;
  const offset = Math.min(top, Math.max(0, rows.length * h - view));
  const first = virtual ? Math.max(0, Math.floor(offset / h) - overscan) : 0;
  const last = virtual ? Math.min(rows.length, Math.ceil((offset + view) / h) + overscan) : rows.length;
  useLayoutEffect(() => {
    const el = scroller.current!;
    const measure = () => setView(el.clientHeight);
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(el);
    return () => watch.disconnect();
  }, []);
  useLayoutEffect(() => { setTop(scroller.current?.scrollTop ?? 0); }, [rows.length, h, virtual]);

  const all = selectable && rows.length > 0 && rows.every((r) => selected.has(rowKey(r)));
  const some = selectable && rows.some((r) => selected.has(rowKey(r)));
  const toggle = (key: string) => {
    if (!selectable) return;
    const next = new Set(selected);
    if (!next.delete(key)) next.add(key);
    onSelect(next);
  };

  return (
    <div ref={scroller} className="st-table" data-fill={fill || undefined} data-dense={dense || undefined} role="table" onScroll={virtual ? (e) => setTop(e.currentTarget.scrollTop) : undefined}>
      <div className="st-table__row st-table__head" role="row" style={{ gridTemplateColumns: template }}>
        {selectable && (
          <div className="st-table__cell" role="columnheader">
            <StCheckbox checked={all} indeterminate={some} onChange={() => onSelect(all ? new Set() : new Set(rows.map(rowKey)))} />
          </div>
        )}
        {columns.map((c) => (
          <div key={c.key} className="st-table__cell" role="columnheader" data-align={c.align}>
            {c.title}
          </div>
        ))}
      </div>
      {rows.length === 0 && !loading && (typeof empty === "string" ? <StEmpty>{empty}</StEmpty> : empty)}
      {virtual && first > 0 && <div style={{ height: first * h, flexShrink: 0 }} />}
      {rows.slice(first, last).map((row, i) => {
        const key = rowKey(row);
        const on = selectable && selected.has(key);
        return (
          <div key={key} className="st-table__row" role="row" style={{ gridTemplateColumns: template }} data-selected={on || undefined} data-clickable={onRowClick ? true : undefined}
            onClick={onRowClick ? () => onRowClick(row) : undefined}>
            {selectable && (
              <div className="st-table__cell" role="cell" onClick={(e) => e.stopPropagation()}>
                <StCheckbox checked={on} onChange={() => toggle(key)} />
              </div>
            )}
            {columns.map((c) => (
              <div key={c.key} className="st-table__cell" role="cell" data-align={c.align}>
                {c.render(row, first + i)}
              </div>
            ))}
          </div>
        );
      })}
      {virtual && last < rows.length && <div style={{ height: (rows.length - last) * h, flexShrink: 0 }} />}
    </div>
  );
}
