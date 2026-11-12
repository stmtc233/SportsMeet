import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StSegmented, StSelect, StTextField } from "../../design";
import type { EventInput, EventKind, EventRow } from "../../mygo";
import { EventsController, useEventPresets, useEventRules } from "./controller";

const blank: Required<EventInput> = { name: "", gender: "男", discipline: "", groupLabel: "", kind: "track", date: "", laneCount: 8, attemptCount: 3, note: "", prelim: false, advanceByPlace: 2, advanceByMark: 2, startMode: "lanes", distanceMode: "", relayLegCount: 0 };

function toInput(row: EventRow): Required<EventInput> {
  const e = row.event;
  return { name: row.autoName ? "" : e.name, gender: e.gender, discipline: e.discipline, groupLabel: e.groupLabel, kind: e.kind, date: e.date, laneCount: e.laneCount, attemptCount: e.attemptCount, note: e.note,
    prelim: row.prelim, advanceByPlace: row.advance?.byPlace ?? 2, advanceByMark: row.advance?.byMark ?? 2,
    startMode: e.startMode, distanceMode: e.distanceMode, relayLegCount: e.relayLegCount };
}

const int = (text: string) => Number.parseInt(text, 10) || 0;

/** Creates an event (row undefined) or edits one. */
export function EventDialog({ meetId, open, row, onClose }: { meetId: string; open: boolean; row?: EventRow; onClose: () => void }) {
  const [form, setForm] = useState<Required<EventInput>>(blank);
  const [presetId, setPresetId] = useState("");
  const [auto, setAuto] = useState("");
  const [busy, setBusy] = useState(false);
  const { data: presets } = useEventPresets();
  const rules = useEventRules(form, open);
  useEffect(() => {
    if (open) { setForm(row ? toInput(row) : blank); setPresetId(""); }
  }, [open, row]);
  useEffect(() => {
    if (!open) return;
    let live = true;
    void EventsController.eventName(form.gender, form.discipline, form.groupLabel).then((n) => { if (live) setAuto(n); }).catch(() => {});
    return () => { live = false; };
  }, [open, form.gender, form.discipline, form.groupLabel]);
  const set = (patch: Partial<Required<EventInput>>) => setForm((f) => ({ ...f, ...patch }));
  const choosePreset = (id: string) => {
    setPresetId(id);
    const preset = presets?.find((p) => p.id === id);
    if (!preset) return;
    const input = preset.input;
    set({ kind: input.kind, discipline: input.discipline, gender: input.gender === "混合" ? "混合" : form.gender,
      startMode: input.startMode ?? "", distanceMode: input.distanceMode ?? "", relayLegCount: input.relayLegCount ?? 0,
      laneCount: input.laneCount ?? 8, attemptCount: input.attemptCount ?? 3 });
  };
  const changeKind = (value: string) => {
    setPresetId("");
    set({ kind: value, startMode: value === "track" || value === "relay" ? "lanes" : "",
      distanceMode: value === "distance" ? "jump" : "", relayLegCount: 0 });
  };
  const save = async (another: boolean) => {
    if (busy) return;
    setBusy(true);
    const ok = await EventsController.save(meetId, row?.event.id ?? "", form);
    setBusy(false);
    if (!ok) return;
    if (another) { set({ name: "", discipline: "" }); setPresetId(""); }
    else onClose();
  };
  const kind = form.kind as EventKind;
  const timed = kind === "track" || kind === "relay";
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
        <StSegmented label="类型" value={form.kind} onChange={changeKind}
          options={[{ value: "track", label: "径赛" }, { value: "relay", label: "接力" }, { value: "distance", label: "远度" }, { value: "height", label: "高度" }]} />
        <StTextField label="项目名" placeholder={kind === "relay" ? "4×100m" : kind === "track" ? "100m" : kind === "distance" ? "跳远" : "跳高"} value={form.discipline} onChange={(discipline) => { setPresetId(""); set({ discipline }); }} autoFocus />
        <StTextField label="组别" placeholder="可不填" value={form.groupLabel} onChange={(groupLabel) => set({ groupLabel })} />
        <div className="span"><StTextField label="显示名称" placeholder={auto} value={form.name} onChange={(name) => set({ name })} width="fill" /></div>
        {timed && <div className="span"><StSegmented label="起跑方式" value={form.startMode || "lanes"} onChange={(startMode) => set({ startMode })}
          options={[{ value: "lanes", label: "分道" }, { value: "group", label: "不分道" }, { value: "break", label: "分道后抢道" }]} /></div>}
        {kind === "distance" && <div className="span"><StSegmented label="远度方式" value={form.distanceMode || "jump"} onChange={(distanceMode) => set({ distanceMode })}
          options={[{ value: "jump", label: "跳跃" }, { value: "throw", label: "投掷" }]} /></div>}
        <StDateField label="日期" value={form.date} onChange={(date) => set({ date })} />
        {timed && <StTextField label="道数" disabled={form.startMode === "group"} inputMode="numeric" value={String(form.laneCount || "")} onChange={(v) => set({ laneCount: int(v) })} />}
        {kind === "distance" && <StTextField label={`${rules?.attemptLabel || "尝试"}次数`} inputMode="numeric" value={String(form.attemptCount || "")} onChange={(v) => set({ attemptCount: int(v) })} />}
        {kind === "relay" && <StTextField label="建议棒数" placeholder="未指定" inputMode="numeric" value={String(form.relayLegCount || "")} onChange={(v) => set({ relayLegCount: int(v) })} />}
        <div className="span"><StSegmented label="赛制" value={form.prelim ? "prelim" : "final"} onChange={(v) => set({ prelim: v === "prelim" })}
          options={[{ value: "final", label: "直接决赛" }, { value: "prelim", label: "预赛 + 决赛" }]} /></div>
        {form.prelim && <>
          <StTextField label="各组前几名晋级" inputMode="numeric" suffix="名" value={String(form.advanceByPlace)} onChange={(v) => set({ advanceByPlace: int(v) })} />
          <StTextField label="另按成绩晋级" inputMode="numeric" suffix="名" value={String(form.advanceByMark)} onChange={(v) => set({ advanceByMark: int(v) })} />
        </>}
      </div>
    </StDialog>
  );
}
