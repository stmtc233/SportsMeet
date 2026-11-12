import { useState } from "react";
import { StButton, StDateField, StDialog, StNotice, StRow, StTimeField } from "../../design";
import { joinScheduled, splitScheduled } from "../../lib/wallTime";
import type { RoundView } from "../../mygo";
import { ArrangeController, useScheduleWarning } from "./controller";

/** Sets when a round starts; heats without a time of their own follow it. No time clears it. */
export function RoundStartDialog({ eventId, round, day, onClose }: { eventId: string; round: Pick<RoundView, "id" | "label" | "scheduledAt">; day: string; onClose: () => void }) {
  const fields = splitScheduled(round.scheduledAt);
  const [date, setDate] = useState(fields.date || day);
  const [time, setTime] = useState(fields.time);
  const [busy, setBusy] = useState(false);
  const scheduledAt = joinScheduled(date, time);
  const warning = useScheduleWarning(eventId, scheduledAt);
  const save = async () => {
    if (busy) return;
    setBusy(true);
    const saved = await ArrangeController.setRoundStart({ eventId, roundId: round.id, scheduledAt });
    setBusy(false);
    if (saved) onClose();
  };
  const close = () => { if (!busy) onClose(); };
  return (
    <StDialog open title={`${round.label}开始时间`} size="small" onClose={close} onSubmit={save}
      actions={
        <>
          <StButton disabled={busy} onClick={close}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
        </>
      }>
      <StRow>
        <StDateField label="日期" value={date} disabled={busy} onChange={setDate} />
        <StTimeField label="时间" value={time} disabled={busy} onChange={setTime} />
      </StRow>
      {warning && <StNotice tone="warning">{warning}</StNotice>}
    </StDialog>
  );
}
