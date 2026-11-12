import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StRow, StTextField, StTimeField } from "../../design";
import type { DraftHeat } from "./draft";

/** Edits a heat's custom name and scheduled time (date and time apart). */
export function HeatDialog({ heat, defaultDate, onClose, onSave }: { heat: DraftHeat | null; defaultDate: string; onClose: () => void; onSave: (patch: { name: string; scheduledAt: string }) => void }) {
  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [time, setTime] = useState("");
  useEffect(() => {
    if (!heat) return;
    const [d, t] = heat.scheduledAt.split("T");
    setName(heat.name);
    setDate(d || defaultDate);
    setTime(t ?? "");
  }, [heat, defaultDate]);
  const save = () => {
    onSave({ name: name.trim(), scheduledAt: time && date ? `${date}T${time}` : "" });
    onClose();
  };
  return (
    <StDialog open={heat !== null} title="编辑组" size="small" onClose={onClose} onSubmit={save}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" type="submit">确定</StButton>
        </>
      }>
      <StTextField label="名称" placeholder="可不填" value={name} onChange={setName} width="fill" autoFocus />
      <StRow>
        <StDateField label="计划日期" value={date} onChange={setDate} />
        <StTimeField label="时间" value={time} onChange={setTime} />
      </StRow>
    </StDialog>
  );
}
