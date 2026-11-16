import { useEffect, useState } from "react";
import { StButton, StCheckbox, StDialog, StMuted, StSegmented, StSelect, StTextField } from "../../design";
import type { Athlete, AthleteEntryOption, AthleteInput, Unit } from "../../mygo";
import { GroupSelect } from "../groups/GroupFields";
import { AthletesController, useAthleteEntryOptions } from "./controller";

type AthleteForm = Required<Omit<AthleteInput, "gender" | "competitionGender" | "genderIdentity">> & { competitionGender: string; genderIdentity: string };

const blank: AthleteForm = { name: "", competitionGender: "", genderIdentity: "", bib: "", studentNumber: "", groupId: "", unitId: "", className: "", grade: "", birthDate: "", note: "" };

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
  const [form, setForm] = useState<AthleteForm>(blank);
  const [busy, setBusy] = useState(false);
  const [allEvents, setAllEvents] = useState(false);
  // 报名项目 as ticked here; undefined until one is changed, then the athlete's entries follow it.
  const [ticked, setTicked] = useState<Set<string>>();
  const { data: options } = useAthleteEntryOptions(meetId, athlete?.id ?? "", form.competitionGender, form.groupId, open);
  const entered = new Set((options ?? []).filter((o) => o.entered).map((o) => o.eventId));
  const events = ticked ?? entered;
  useEffect(() => {
    if (!open) return;
    setTicked(undefined);
    setAllEvents(false);
    setForm(athlete ? { name: athlete.name, competitionGender: athlete.competitionGender, genderIdentity: athlete.genderIdentity, bib: athlete.bib, studentNumber: athlete.studentNumber, groupId: athlete.groupId ?? "", unitId: athlete.unitId, className: athlete.className, grade: athlete.grade, birthDate: athlete.birthDate, note: athlete.note } : { ...blank, unitId, groupId });
  }, [open, athlete, unitId, groupId]);
  const set = (patch: Partial<AthleteForm>) => setForm((f) => ({ ...f, ...patch }));
  const unitGroups = (id: string) => units.find((u) => u.id === id)?.groupIds ?? [];
  const save = async (another: boolean) => {
    setBusy(true);
    const ok = await AthletesController.save(meetId, athlete?.id ?? "", form);
    // All choices stay loaded, including selected events outside the current filter.
    const ids = (options ?? []).filter((o) => events.has(o.eventId)).map((o) => o.eventId);
    const id = ok ? ok.athlete?.id : undefined;
    const entriesOk = !ok || !id || ticked === undefined || (await AthletesController.setEntries(meetId, id, ids));
    setBusy(false);
    if (!ok || !entriesOk) return;
    setTicked(undefined);
    if (another) setForm((f) => ({ ...blank, competitionGender: f.competitionGender, unitId: f.unitId, className: f.className, grade: f.grade, groupId: f.groupId }));
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
        <StSelect label="性别认同" value={form.genderIdentity} onChange={(genderIdentity) => set({ genderIdentity })}
          options={[{ value: "", label: "未填" }, { value: "男", label: "男" }, { value: "女", label: "女" }, { value: "非二元", label: "非二元" }, { value: "其他", label: "其他" }, { value: "不愿透露", label: "不愿透露" }]} />
        <StTextField label="号码" value={form.bib} onChange={(bib) => set({ bib })} />
        <StTextField label="学号" value={form.studentNumber} onChange={(studentNumber) => set({ studentNumber })} />
        <div className="span">
          <StSelect label="参赛单位" width="fill" value={form.unitId} onChange={(unitId) => set({ unitId, groupId: groupIn(unitGroups(unitId), form.groupId) })} placeholder="无"
            options={[{ value: "", label: "无" }, ...units.map((u) => ({ value: u.id, label: u.name }))]} />
        </div>
        <GroupSelect meetId={meetId} value={form.groupId} onChange={(groupId) => set({ groupId })} required={unitGroups(form.unitId).length === 1} />
        <StTextField label="年级" value={form.grade} onChange={(grade) => set({ grade })} />
        <StTextField label="班级" value={form.className} onChange={(className) => set({ className })} />
      </div>
      <StSegmented label="参赛类别" value={form.competitionGender} onChange={(competitionGender) => set({ competitionGender })}
        options={[{ value: "男", label: "男子" }, { value: "女", label: "女子" }, { value: "", label: "未指定" }]} />
      <StCheckbox label="查看全部项目" checked={allEvents} onChange={setAllEvents} />
      <EntryChecks options={options?.filter((o) => allEvents || o.matches || events.has(o.eventId))} ticked={events} onChange={setTicked} />
    </StDialog>
  );
}

/**
 * The group an athlete takes moving into a unit of groups: the unit's one
 * group, the athlete's while the unit has it, else none.
 */
function groupIn(groups: string[], group: string) {
  if (groups.length === 1) return groups[0]!;
  return groups.includes(group) ? group : "";
}

/** 报名项目: the individual events, ticked when entered; one in a heat stays ticked. */
function EntryChecks({ options, ticked, onChange }: { options: AthleteEntryOption[] | undefined; ticked: Set<string>; onChange: (next: Set<string>) => void }) {
  if (!options) return null;
  const toggle = (id: string) => {
    const next = new Set(ticked);
    if (!next.delete(id)) next.add(id);
    onChange(next);
  };
  return (
    <fieldset className="entry-checks">
      <legend className="entry-checks__title">报名项目</legend>
      {options.length === 0 ? <StMuted>没有可报的个人项目</StMuted> : (
        <div className="entry-checks__list">
          {options.map((o) => (
            <StCheckbox key={o.eventId} label={o.name} checked={ticked.has(o.eventId)} disabled={o.placed && o.entered} onChange={() => toggle(o.eventId)} />
          ))}
        </div>
      )}
    </fieldset>
  );
}
