import { useEffect, useState } from "react";
import { StButton, StDateField, StDialog, StSegmented, StTextField } from "../../design";
import { Marks, type EventInput, type EventKind, type EventRow, type Gender } from "../../mygo";
import { EventsController } from "./controller";

const blank: EventInput = { name: "", gender: "男", discipline: "", groupLabel: "", kind: "track", date: "", laneCount: 8, attemptCount: 3, note: "", prelim: false, advanceByPlace: 2, advanceByMark: 2 };

function toInput(row: EventRow): EventInput {
  const e = row.event;
  return { name: row.autoName ? "" : e.name, gender: e.gender, discipline: e.discipline, groupLabel: e.groupLabel, kind: e.kind, date: e.date, laneCount: e.laneCount, attemptCount: e.attemptCount, note: e.note,
    prelim: row.prelim, advanceByPlace: row.advance?.byPlace ?? 2, advanceByMark: row.advance?.byMark ?? 2 };
}

const int = (text: string) => Number.parseInt(text, 10) || 0;

/** Creates an event (row undefined) or edits one. */
export function EventDialog({ meetId, open, row, onClose }: { meetId: string; open: boolean; row?: EventRow; onClose: () => void }) {
  const [form, setForm] = useState<EventInput>(blank);
  const [auto, setAuto] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) setForm(row ? toInput(row) : blank);
  }, [open, row]);
  // The composed name comes from Go, the only place that rule lives.
  useEffect(() => {
    let live = true;
    void Marks.eventName(form.gender as Gender, form.discipline, form.groupLabel).then((n) => live && setAuto(n));
    return () => {
      live = false;
    };
  }, [form.gender, form.discipline, form.groupLabel]);
  const set = (patch: Partial<EventInput>) => setForm((f) => ({ ...f, ...patch }));
  const save = async (another: boolean) => {
    setBusy(true);
    const ok = await EventsController.save(meetId, row?.event.id ?? "", form);
    setBusy(false);
    if (!ok) return;
    if (another) set({ name: "", discipline: "" });
    else onClose();
  };
  const kind = form.kind as EventKind;
  return (
    <StDialog open={open} title={row ? "编辑项目" : "添加项目"} onClose={onClose} onSubmit={() => save(false)}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          {!row && <StButton loading={busy} onClick={() => save(true)}>保存并继续</StButton>}
          <StButton variant="primary" type="submit" loading={busy}>保存</StButton>
        </>
      }>
      <div className="form-grid">
        <StSegmented label="性别" value={form.gender} onChange={(gender) => set({ gender })}
          options={[{ value: "男", label: "男" }, { value: "女", label: "女" }, { value: "混合", label: "混合" }]} />
        <StSegmented label="类型" value={form.kind} onChange={(k) => set({ kind: k })}
          options={[{ value: "track", label: "径赛" }, { value: "distance", label: "远度" }, { value: "height", label: "高度" }]} />
        <StTextField label="项目名" placeholder={kind === "track" ? "100m" : kind === "distance" ? "跳远" : "跳高"} value={form.discipline} onChange={(discipline) => set({ discipline })} autoFocus />
        <StTextField label="组别" placeholder="可不填" value={form.groupLabel} onChange={(groupLabel) => set({ groupLabel })} />
        <div className="span">
          <StTextField label="显示名称" placeholder={auto} value={form.name} onChange={(name) => set({ name })} width="fill" />
        </div>
        <StDateField label="日期" value={form.date} onChange={(date) => set({ date })} />
        {kind === "track" && <StTextField label="道数" inputMode="numeric" value={String(form.laneCount || "")} onChange={(v) => set({ laneCount: int(v) })} />}
        {kind === "distance" && <StTextField label="试跳次数" inputMode="numeric" value={String(form.attemptCount || "")} onChange={(v) => set({ attemptCount: int(v) })} />}
        <div className="span">
          <StSegmented label="赛制" value={form.prelim ? "prelim" : "final"} onChange={(v) => set({ prelim: v === "prelim" })}
            options={[{ value: "final", label: "直接决赛" }, { value: "prelim", label: "预赛 + 决赛" }]} />
        </div>
        {form.prelim && (
          <>
            <StTextField label="各组前几名晋级" inputMode="numeric" suffix="名" value={String(form.advanceByPlace)} onChange={(v) => set({ advanceByPlace: int(v) })} />
            <StTextField label="另按成绩晋级" inputMode="numeric" suffix="名" value={String(form.advanceByMark)} onChange={(v) => set({ advanceByMark: int(v) })} />
          </>
        )}
      </div>
    </StDialog>
  );
}
