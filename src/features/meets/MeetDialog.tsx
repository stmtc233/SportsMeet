import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StTextField } from "../../design";
import type { Meet, MeetInput } from "../../mygo";
import { MeetsController } from "./controller";

const blank: MeetInput = { name: "", startDate: "", endDate: "", location: "", note: "" };

function today(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** Creates a meet (meet undefined) or edits one. */
export function MeetDialog({ open, meet, onClose, onSaved }: { open: boolean; meet?: Meet; onClose: () => void; onSaved?: (id: string) => void }) {
  const [form, setForm] = useState<MeetInput>(blank);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm(meet ? { name: meet.name, startDate: meet.startDate, endDate: meet.endDate, location: meet.location, note: meet.note } : { ...blank, startDate: today() });
  }, [open, meet]);
  const set = (patch: Partial<MeetInput>) => setForm((f) => ({ ...f, ...patch }));
  const save = async () => {
    setBusy(true);
    const id = await MeetsController.save(meet?.id ?? "", form);
    setBusy(false);
    if (id) {
      onClose();
      onSaved?.(id);
    }
  };
  return (
    <StDialog open={open} title={meet ? "编辑赛事" : "新建赛事"} onClose={onClose} onSubmit={save}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
        </>
      }>
      <div className="form-grid">
        <div className="span">
          <StTextField label="赛事名称" value={form.name} onChange={(name) => set({ name })} width="fill" autoFocus />
        </div>
        <StDateField label="开始日期" value={form.startDate} onChange={(startDate) => set({ startDate })} />
        <StDateField label="结束日期" value={form.endDate} onChange={(endDate) => set({ endDate })} />
        <div className="span">
          <StTextField label="地点" value={form.location} onChange={(location) => set({ location })} width="fill" />
        </div>
      </div>
    </StDialog>
  );
}
