import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StTextField } from "../../design";
import type { EntryLimits, Meet, MeetInput } from "../../mygo";
import { today } from "../../lib/wallTime";
import { MeetsController } from "./controller";
import { limitText, limitValue, noLimits } from "./limits";

const blank: Required<MeetInput> = { name: "", startDate: "", endDate: "", location: "", note: "", entryLimits: noLimits };
const limitFields: { key: keyof EntryLimits; label: string; unit: string }[] = [
  { key: "athleteEvents", label: "每人限报项数", unit: "项" },
  { key: "unitAthletes", label: "单位每项限报人数", unit: "人" },
  { key: "unitTeams", label: "单位每项限报队数", unit: "队" },
];

/** Creates a meet (meet undefined) or edits one. */
export function MeetDialog({ open, meet, onClose, onSaved }: { open: boolean; meet?: Meet; onClose: () => void; onSaved?: (id: string) => void }) {
  const [form, setForm] = useState<Required<MeetInput>>(blank);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm(meet ? { name: meet.name, startDate: meet.startDate, endDate: meet.endDate, location: meet.location, note: meet.note, entryLimits: meet.entryLimits ?? noLimits } : { ...blank, startDate: today() });
  }, [open, meet]);
  const set = (patch: Partial<Required<MeetInput>>) => setForm((f) => ({ ...f, ...patch }));
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
        {limitFields.map(({ key, label, unit }) => (
          <StTextField key={key} label={label} inputMode="numeric" placeholder="不限" suffix={unit} value={limitText(form.entryLimits[key])}
            onChange={(text) => set({ entryLimits: { ...form.entryLimits, [key]: limitValue(text) } })} />
        ))}
      </div>
    </StDialog>
  );
}
