import { Fragment, useEffect, useState } from "react";
import { StButton, StDialog, StSegmented, StSelect, StTextField } from "../../design";
import type { EventInput, EventKind, EventRow, Stage } from "../../mygo";
import { stageLabels } from "../../lib/labels";
import { useGroups } from "../groups/controller";
import { GroupSelect } from "../groups/GroupFields";
import { EventsController, useEventPresets } from "./controller";

import { formatKey, formatOptions, withFormat, withRound } from "./format";
import { blankEvent as blank, EventMeasurementFields } from "./EventMeasurementFields";

function toInput(row: EventRow): Required<EventInput> {
  const e = row.event;
  return { ...blank, participantMode: e.participantMode || (e.kind === "relay" ? "team" : "athlete"), teamSize: e.teamSize ?? 0, scoreDecimals: e.scoreDecimals ?? 0, rankDirection: e.rankDirection ?? "", attemptMode: e.attemptMode ?? "", name: row.autoName ? "" : e.name, gender: e.gender, discipline: e.discipline, groupId: e.groupId ?? "", groupLabel: e.groupLabel, kind: e.kind, date: e.date, laneCount: e.laneCount, attemptCount: e.attemptCount, note: e.note,
    rounds: row.rounds,
    startMode: e.startMode, distanceMode: e.distanceMode, relayLegCount: e.relayLegCount };
}

const int = (text: string) => Number.parseInt(text, 10) || 0;

/** Creates an event (row undefined) or edits one. */
export function EventDialog({ meetId, open, row, initialGroupId = "", onClose }: { meetId: string; open: boolean; row?: EventRow; initialGroupId?: string; onClose: () => void }) {
  const { data: groups } = useGroups(meetId);
  const groupName = (id: string) => groups?.find((g) => g.id === id)?.name;
  const [form, setForm] = useState<Required<EventInput>>(blank);
  const [presetId, setPresetId] = useState("");
  const [auto, setAuto] = useState("");
  const [busy, setBusy] = useState(false);
  const { data: presets } = useEventPresets();
  useEffect(() => {
    if (open) { setForm(row ? toInput(row) : { ...blank, groupId: initialGroupId }); setPresetId(""); }
  }, [open, row, initialGroupId]);
  useEffect(() => {
    if (!open) return;
    let live = true;
    void EventsController.eventName(form.gender, form.discipline, groupName(form.groupId) ?? form.groupLabel).then((n) => { if (live) setAuto(n); }).catch(() => {});
    return () => { live = false; };
  }, [open, form.gender, form.discipline, form.groupLabel, form.groupId, groups]);
  const set = (patch: Partial<Required<EventInput>>) => setForm((f) => ({ ...f, ...patch }));
  const choosePreset = (id: string) => {
    setPresetId(id);
    const preset = presets?.find((p) => p.id === id);
    if (!preset) return;
    const input = preset.input;
    set({ kind: input.kind, discipline: input.discipline, gender: input.gender === "混合" ? "混合" : form.gender,
      startMode: input.startMode ?? "", distanceMode: input.distanceMode ?? "", relayLegCount: input.relayLegCount ?? 0,
      laneCount: input.laneCount ?? 8, attemptCount: input.attemptCount ?? 3, participantMode: input.participantMode ?? (input.kind === "relay" ? "team" : "athlete"), teamSize: input.teamSize ?? 0, scoreDecimals: input.scoreDecimals ?? 0, rankDirection: input.rankDirection ?? "", attemptMode: input.attemptMode ?? "" });
  };
  const save = async (another: boolean) => {
    if (busy) return;
    setBusy(true);
    const ok = await EventsController.save(meetId, row?.event.id ?? "", { ...form, groupLabel: form.groupId ? "" : form.groupLabel });
    setBusy(false);
    if (!ok) return;
    if (another) { set({ name: "", discipline: "" }); setPresetId(""); }
    else onClose();
  };
  const kind = form.kind as EventKind;

  return (
    <StDialog open={open} title={row ? "编辑项目" : "添加项目"} onClose={() => !busy && onClose()} onSubmit={() => save(false)}
      actions={<>
        <StButton disabled={busy} onClick={onClose}>取消</StButton>
        {!row && <StButton loading={busy} onClick={() => save(true)}>保存并继续</StButton>}
        <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
      </>}>
      <div className="form-grid" inert={busy}>
        {!row && <div className="span"><StSelect label="常用项目" value={presetId} onChange={choosePreset} width="fill"
          options={[{ value: "", label: "自定义" }, ...(presets ?? []).map((p) => ({ value: p.id, label: `${p.category} · ${p.label}` }))]} /></div>}
        <StSegmented label="性别" value={form.gender} onChange={(gender) => set({ gender })}
          options={[{ value: "男", label: "男" }, { value: "女", label: "女" }, { value: "混合", label: "混合" }]} />
        <StTextField label="项目名" placeholder={kind === "relay" ? "4×100m" : kind === "track" ? "100m" : kind === "distance" ? "跳远" : "跳高"} value={form.discipline} onChange={(discipline) => { setPresetId(""); set({ discipline }); }} autoFocus />
        <GroupSelect meetId={meetId} value={form.groupId} onChange={(groupId) => set({ groupId, groupLabel: "" })} />
        <div className="span"><StTextField label="显示名称" placeholder={auto} value={form.name} onChange={(name) => set({ name })} width="fill" /></div>
        <EventMeasurementFields form={form} onChange={(patch) => { setPresetId(""); set(patch); }} />
        <div className="span"><StSelect label="赛制" value={formatKey(form.rounds)} onChange={(key) => set({ rounds: withFormat(form.rounds, key) })} width="fill"
          options={formatOptions(row?.rounds ?? [])} /></div>
        {form.rounds.map((r) => {
          const label = stageLabels[r.stage as Stage];
          return (
            <Fragment key={r.stage}>
              <StTextField label={`${label}各组前几名晋级`} inputMode="numeric" suffix="名" value={String(r.advanceByPlace)} onChange={(v) => set({ rounds: withRound(form.rounds, r.stage, { advanceByPlace: int(v) }) })} />
              <StTextField label={`${label}另按成绩晋级`} inputMode="numeric" suffix="名" value={String(r.advanceByMark)} onChange={(v) => set({ rounds: withRound(form.rounds, r.stage, { advanceByMark: int(v) }) })} />
            </Fragment>
          );
        })}
      </div>
    </StDialog>
  );
}
