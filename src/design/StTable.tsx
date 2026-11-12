import { useRef, useState, type ReactNode } from "react";
import { StCheckbox } from "./StFields";

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
  empty?: ReactNode;
  /** Fills its container and scrolls inside, the header staying on top. */
  fill?: boolean;
  dense?: boolean;
}

const rowHeight = { normal: 40, dense: 34 };
const overscan = 12;

/**
 * A data table on a CSS grid. The header sticks to the top; long lists
 * render only the rows in view (rows have a fixed height).
 */
export function StTable<T>({ columns, rows, rowKey, onRowClick, selected, onSelect, empty, fill, dense }: StTableProps<T>) {
  const scroller = useRef<HTMLDivElement>(null);
  const [top, setTop] = useState(0);
  const h = dense ? rowHeight.dense : rowHeight.normal;
  const selectable = selected !== undefined && onSelect !== undefined;
  const template = [selectable ? "40px" : "", ...columns.map((c) => c.width ?? "1fr")].filter(Boolean).join(" ");

  const virtual = fill && rows.length > 200;
  const view = scroller.current?.clientHeight ?? 800;
  const first = virtual ? Math.max(0, Math.floor(top / h) - overscan) : 0;
  const last = virtual ? Math.min(rows.length, Math.ceil((top + view) / h) + overscan) : rows.length;

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
      {rows.length === 0 && empty}
      {virtual && first > 0 && <div style={{ height: first * h }} />}
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
      {virtual && last < rows.length && <div style={{ height: (rows.length - last) * h }} />}
    </div>
  );
}
