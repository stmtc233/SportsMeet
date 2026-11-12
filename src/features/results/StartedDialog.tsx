import { useState } from "react";
import { StButton, StDateField, StDialog, StRow, StTimeField } from "../../design";
import { joinScheduled } from "../../lib/wallTime";
import type { HeatView } from "../../mygo";
import { ResultsController } from "./controller";
import { startedFields } from "./started";

/** Changes or clears when a heat actually started; it was recorded with its first results. */
export function StartedDialog({ heat, day, onClose }: { heat: HeatView; day: string; onClose: () => void }) {
  const fields = startedFields(heat, day);
  const [date, setDate] = useState(fields.date);
  const [time, setTime] = useState(fields.time);
  const [busy, setBusy] = useState(false);
  const startedAt = joinScheduled(date, time);
  const write = async (value: string) => {
    if (busy) return;
    setBusy(true);
    const saved = await ResultsController.setStarted({ heatId: heat.id, startedAt: value });
    setBusy(false);
    if (saved) onClose();
  };
  const close = () => { if (!busy) onClose(); };
  return (
    <StDialog open title={`${heat.label}开赛时间`} size="small" onClose={close} onSubmit={() => startedAt && write(startedAt)}
      actions={
        <>
          {heat.startedAt && <StButton disabled={busy} onClick={() => write("")}>清除</StButton>}
          <StButton disabled={busy} onClick={close}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy} disabled={!startedAt}>保存</StButton>
        </>
      }>
      <StRow>
        <StDateField label="日期" value={date} disabled={busy} onChange={setDate} />
        <StTimeField label="时间" value={time} disabled={busy} onChange={setTime} />
      </StRow>
    </StDialog>
  );
}
