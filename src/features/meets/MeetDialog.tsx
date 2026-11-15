import { useEffect, useState } from "react";
import { StButton, StDialog, StMuted, StSelect, StStack } from "../../design";
import type { Meet, MeetInput, MeetView } from "../../mygo";
import { today } from "../../lib/wallTime";
import { MeetsController } from "./controller";
import { formOf } from "./limits";
import { MeetInfoFields, MeetLimitFields } from "./MeetFields";

/** Creates a meet (meet undefined) or edits one. */
export function MeetDialog({ open, meet, sources = [], onClose, onSaved }: { open: boolean; meet?: Meet; sources?: MeetView[]; onClose: () => void; onSaved?: (id: string) => void }) {
  const [form, setForm] = useState<Required<MeetInput>>(() => formOf());
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm(meet ? formOf(meet) : { ...formOf(), startDate: today() });
  }, [open, meet]);
  const set = (patch: Partial<Required<MeetInput>>) => setForm((f) => ({ ...f, ...patch }));
  const save = async () => {
    if (busy) return;
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
      <StStack gap={12}>
        <MeetInfoFields form={form} onChange={set} autoFocus />
        {!meet && sources.length > 0 && <>
          <StSelect label="沿用项目与赛制" value={form.sourceMeetId} onChange={(sourceMeetId) => set({ sourceMeetId })} width="fill" disabled={busy}
            options={[{ value: "", label: "不沿用" }, ...sources.map((m) => ({ value: m.id, label: `${m.name} · ${m.datesText}` }))]} />
          {form.sourceMeetId && <StMuted>沿用组别和计分规则，不含名单、编排与时间。</StMuted>}
        </>}
        <MeetLimitFields form={form} onChange={set} />
      </StStack>
    </StDialog>
  );
}
