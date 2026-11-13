import type { HeatView, ResultInput } from "../../mygo";
import { toRow, type Rows } from "./rows";

export function heightColumns(heat: HeatView, rows: Rows, extra: string[], removed: string[]) {
  const all = new Set(extra);
  for (const slot of heat.slots) for (const bar of slot.result?.bars ?? []) all.add(bar.heightText);
  for (const row of Object.values(rows)) for (const [height, trials] of Object.entries(row.bars)) if (trials.trim()) all.add(height);
  return [...all].filter((height) => !removed.includes(height)).sort((a, b) => Number(a) - Number(b));
}

export function heightInputs(heat: HeatView, rows: Rows, heights: string[], finished = false): ResultInput[] {
  return heat.slots.map((slot) => {
    const typed = rows[slot.position];
    const row = typed?.competitorId === slot.competitorId ? typed : toRow(slot, 0);
    return { position: slot.position, status: row.status, heightFinished: finished,
      // Height marks without trials from exchange files remain editable as records.
      mark: slot.result && slot.result.bars.length === 0 ? row.mark : "",
      bars: heights.map((height) => ({ height, trials: row.bars[height] ?? "" })), note: slot.result?.note ?? "" };
  });
}

/** The trial a key records in a height cell; Go normalizes typed records. */
export function trialSymbol(key: string): "O" | "X" | "-" | undefined {
  if (key === "o" || key === "O" || key === "0") return "O";
  if (key === "x" || key === "X" || key === "×") return "X";
  if (key === "-" || key === "—" || key === "－") return "-";
  return undefined;
}
