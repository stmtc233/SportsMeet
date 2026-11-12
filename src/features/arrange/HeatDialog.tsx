import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StMuted, StNotice, StRow, StTextField, StTimeField } from "../../design";
import { joinScheduled, splitScheduled } from "../../lib/wallTime";
import { ArrangeController, useScheduleWarning } from "./controller";
import type { DraftHeat, DraftRound, HeatPatch } from "./draft";

/**
 * Edits a heat's custom name, own time (date and time apart; no time
 * follows the round's start) and expected duration. day is the event's
 * shown day; Go checks the fields and returns their texts.
 */
export function HeatDialog({ heat, round, eventId, day, onClose, onSave }: {
  heat: Pick<DraftHeat, "name" | "scheduledAt" | "durationMinutes"> | null;
  round: Pick<DraftRound, "startAt" | "startText">;
  eventId: string;
  day: string;
  onClose: () => void;
  onSave: (patch: HeatPatch) => void;
}) {
  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [time, setTime] = useState("");
  const [minutes, setMinutes] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!heat) return;
    const fields = splitScheduled(heat.scheduledAt);
    setName(heat.name);
    setDate(fields.date || splitScheduled(round.startAt).date || day);
    setTime(fields.time);
    setMinutes(heat.durationMinutes === null ? "" : String(heat.durationMinutes));
    setBusy(false);
    // Opening the dialog reads the heat once; later round changes keep the typed fields.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [heat]);
  const scheduledAt = joinScheduled(date, time);
  const warning = useScheduleWarning(eventId, scheduledAt);
  const save = async () => {
    setBusy(true);
    const plan = await ArrangeController.heatPlan(round.startAt, scheduledAt, minutes, day);
    setBusy(false);
    if (typeof plan !== "object" || plan === null) return;
    onSave({ name: name.trim(), scheduledAt: plan.scheduledAt, durationMinutes: plan.durationMinutes,
      scheduledText: plan.scheduledText, scheduleInherited: plan.scheduleInherited, durationText: plan.durationText });
    onClose();
  };
  return (
    <StDialog open={heat !== null} title="编辑组" size="small" onClose={onClose} onSubmit={save}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy}>确定</StButton>
        </>
      }>
      <StTextField label="名称" placeholder="可不填" value={name} onChange={setName} width="fill" autoFocus />
      <StRow>
        <StDateField label="计划日期" value={date} onChange={setDate} />
        <StTimeField label="时间" value={time} onChange={setTime} />
        <StTextField label="预计用时" inputMode="numeric" width="narrow" suffix="分钟" placeholder="可不填" value={minutes} onChange={setMinutes} />
      </StRow>
      {!time && round.startText && <StMuted>沿用轮次 {round.startText}</StMuted>}
      {warning && <StNotice tone="warning">{warning}</StNotice>}
    </StDialog>
  );
}
