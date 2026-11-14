import { useState } from "react";
import { useLeaveGuard } from "../../app/nav";
import { StButton, StSection } from "../../design";
import type { MeetInput } from "../../mygo";
import { MeetsController } from "../meets/controller";
import { useMeet } from "../meets/data";
import { formOf, sameForm } from "../meets/limits";
import { MeetInfoFields, MeetLimitFields } from "../meets/MeetFields";

/** The meet's name, dates, place and 报名限制, saved together from the bar below them. */
export function MeetSection({ meetId }: { meetId: string }) {
  const { data: meet, reload } = useMeet(meetId);
  // Only the fields typed into; a reload of the meet shows through the others.
  const [draft, setDraft] = useState<Partial<Required<MeetInput>> | null>(null);
  const [busy, setBusy] = useState(false);
  const saved = meet ? formOf(meet) : null;
  const form = { ...(saved ?? formOf()), ...draft };
  const dirty = draft !== null && saved !== null && !sameForm(form, saved);
  useLeaveGuard(dirty);
  const set = (patch: Partial<Required<MeetInput>>) => setDraft((d) => ({ ...d, ...patch }));
  const save = async () => {
    setBusy(true);
    const ok = await MeetsController.save(meetId, form);
    // The draft gives way once the saved meet is read back, so the old values never show in between.
    if (ok) {
      await reload();
      setDraft(null);
    }
    setBusy(false);
  };
  return (
    <>
      <StSection title="赛事信息">
        <div inert={!meet || busy}><MeetInfoFields form={form} onChange={set} /></div>
      </StSection>
      <StSection title="报名限制">
        <div inert={!meet || busy}><MeetLimitFields form={form} onChange={set} /></div>
      </StSection>
      {dirty && (
        <div className="save-bar">
          <StButton disabled={busy} onClick={() => setDraft(null)}>放弃</StButton>
          <StButton variant="primary" loading={busy} onClick={save}>保存</StButton>
        </div>
      )}
    </>
  );
}
