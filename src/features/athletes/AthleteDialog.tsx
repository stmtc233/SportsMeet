import { useEffect, useState } from "react";
import { StButton, StDialog, StSegmented, StSelect, StTextField } from "../../design";
import type { Athlete, AthleteInput, Unit } from "../../mygo";
import { GroupSelect } from "../groups/GroupFields";
import { AthletesController } from "./controller";

const blank: Required<AthleteInput> = { name: "", gender: "", bib: "", studentNumber: "", groupId: "", unitId: "", className: "", grade: "", birthDate: "", note: "" };

/** Creates an athlete (athlete undefined; in unitId and groupId when given) or edits one. */
export function AthleteDialog({ meetId, open, athlete, units, unitId = "", groupId = "", onClose }: {
  meetId: string;
  open: boolean;
  athlete?: Athlete;
  units: Unit[];
  unitId?: string;
  groupId?: string;
  onClose: () => void;
}) {
  const [form, setForm] = useState<Required<AthleteInput>>(blank);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!open) return;
    setForm(athlete ? { name: athlete.name, gender: athlete.gender, bib: athlete.bib, studentNumber: athlete.studentNumber, groupId: athlete.groupId ?? "", unitId: athlete.unitId, className: athlete.className, grade: athlete.grade, birthDate: athlete.birthDate, note: athlete.note } : { ...blank, unitId, groupId });
  }, [open, athlete, unitId, groupId]);
  const set = (patch: Partial<Required<AthleteInput>>) => setForm((f) => ({ ...f, ...patch }));
  const save = async (another: boolean) => {
    setBusy(true);
    const ok = await AthletesController.save(meetId, athlete?.id ?? "", form);
    setBusy(false);
    if (!ok) return;
    if (another) setForm((f) => ({ ...blank, gender: f.gender, unitId: f.unitId, className: f.className, grade: f.grade, groupId: f.groupId }));
    else onClose();
  };
  return (
    <StDialog open={open} title={athlete ? "编辑运动员" : "添加运动员"} onClose={onClose} onSubmit={() => save(false)}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          {!athlete && <StButton loading={busy} onClick={() => save(true)}>保存并继续</StButton>}
          <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
        </>
      }>
      <div className="form-grid">
        <StTextField label="姓名" value={form.name} onChange={(name) => set({ name })} autoFocus />
        <StSegmented label="性别" value={form.gender} onChange={(gender) => set({ gender })}
          options={[{ value: "男", label: "男" }, { value: "女", label: "女" }, { value: "", label: "未填" }]} />
        <StTextField label="号码" value={form.bib} onChange={(bib) => set({ bib })} />
        <StTextField label="学号" value={form.studentNumber} onChange={(studentNumber) => set({ studentNumber })} />
        <div className="span">
          <StSelect label="参赛单位" width="fill" value={form.unitId} onChange={(unitId) => { const unit = units.find((u) => u.id === unitId); set({ unitId, ...(!form.groupId && unit?.groupIds?.length === 1 ? { groupId: unit.groupIds[0]! } : {}) }); }} placeholder="无"
            options={[{ value: "", label: "无" }, ...units.map((u) => ({ value: u.id, label: u.name }))]} />
        </div>
        <GroupSelect meetId={meetId} value={form.groupId} onChange={(groupId) => set({ groupId })} />
        <StTextField label="年级" value={form.grade} onChange={(grade) => set({ grade })} />
        <StTextField label="班级" value={form.className} onChange={(className) => set({ className })} />
      </div>
    </StDialog>
  );
}
