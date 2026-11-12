import { useEffect, useRef, useState } from "react";
import { StButton, StDateField, StDialog, StEmpty, StNotice, StRow, StSection, StTable, StTextField, StTimeField } from "../../design";
import type { EventDetail, RoundView, ScheduleChange, SchedulePreview, ScheduleRequest, ScheduleSkip } from "../../mygo";
import { useMeet } from "../meets/data";
import { ScheduleController } from "./scheduleController";

/** The plan and the confirmed write both come from Go, including all time arithmetic. */
export function ScheduleDialog({ detail, round, onClose }: { detail: EventDetail; round: RoundView; onClose: () => void }) {
  const { data: meet } = useMeet(detail.event.meetId);
  const first = round.heats.find((h) => !h.keepReason)?.scheduledAt ?? "";
  const [date, setDate] = useState(first.split("T")[0] || detail.event.date);
  const [time, setTime] = useState(first.split("T")[1] || "09:00");
  const [interval, setInterval] = useState("5");
  const [preview, setPreview] = useState<SchedulePreview | null>(null);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const epoch = useRef(0);
  const saving = useRef(false);
  const seen = useRef(detail);
  useEffect(() => () => { epoch.current++; }, []);
  useEffect(() => { if (meet) setDate((current) => current || meet.startDate); }, [meet]);
  useEffect(() => {
    if (seen.current === detail) return;
    seen.current = detail;
    if (saving.current) return;
    epoch.current++;
    setBusy(false);
    setPreview(null);
    setNotice("数据已变化，请重新预览");
  }, [detail]);
  const change = (set: (value: string) => void, value: string) => {
    epoch.current++;
    set(value);
    setPreview(null);
    setNotice("");
  };
  const request: ScheduleRequest = { eventId: detail.event.id, roundId: round.id, startAt: `${date}T${time}`, intervalMinutes: Number(interval) };
  const ready = preview !== null && preview.errors.length === 0;
  const run = async () => {
    if (busy) return;
    const mine = ++epoch.current;
    setBusy(true);
    setNotice("");
    if (ready) {
      if (preview.changes.length === 0) { setBusy(false); return; }
      saving.current = true;
      const saved = await ScheduleController.save(request, preview.previewHash);
      saving.current = false;
      if (mine !== epoch.current) return;
      setBusy(false);
      if (saved) onClose();
      else { setPreview(null); setNotice("未能保存，请重新预览"); }
    } else {
      const next = await ScheduleController.preview(request);
      if (mine !== epoch.current) return;
      setBusy(false);
      setPreview(next ?? null);
      if (!next) setNotice("未能预览，请重试");
    }
  };
  const close = () => { if (!busy) onClose(); };
  return (
    <StDialog open title="批量排时间" onClose={close} onSubmit={run}
      actions={<>
        <StButton disabled={busy} onClick={close}>取消</StButton>
        <StButton variant="primary" type="submit" loading={busy} disabled={ready && preview.changes.length === 0} reserveWidthFor={["预览", "保存"]}>{ready ? "保存" : "预览"}</StButton>
      </>}>
      <StRow>
        <StDateField label="首组日期" value={date} disabled={busy} onChange={(v) => change(setDate, v)} />
        <StTimeField label="首组时间" value={time} disabled={busy} onChange={(v) => change(setTime, v)} />
        <StTextField label="间隔" inputMode="numeric" width="narrow" suffix="分钟" value={interval} disabled={busy} onChange={(v) => change(setInterval, v)} />
      </StRow>
      {notice && <StNotice tone="warning">{notice}</StNotice>}
      {preview?.errors.map((error) => <StNotice key={error} tone="danger">{error}</StNotice>)}
      {ready && <StSection title={`修改 ${preview.changes.length} 组`}>
        <div className="preview-table schedule-table">
          <StTable<ScheduleChange> dense fill rows={preview.changes} rowKey={(row) => row.heatId}
            empty={<StEmpty>没有需要更改的时间</StEmpty>}
            columns={[
              { key: "group", title: "组", render: (row) => <span title={row.label}>{row.label}</span> },
              { key: "before", title: "原时间", width: "2fr", render: (row) => <span title={row.beforeText}>{row.beforeText}</span> },
              { key: "after", title: "新时间", width: "2fr", render: (row) => <span title={row.afterText}>{row.afterText}</span> },
            ]} />
        </div>
      </StSection>}
      {ready && preview.skipped.length > 0 && <StSection title={`跳过 ${preview.skipped.length} 组`}>
        <div className="preview-table schedule-table">
          <StTable<ScheduleSkip> dense fill rows={preview.skipped} rowKey={(row) => row.heatId} columns={[
            { key: "group", title: "组", render: (row) => <span title={row.label}>{row.label}</span> },
            { key: "reason", title: "原因", render: (row) => row.reason },
          ]} />
        </div>
      </StSection>}
    </StDialog>
  );
}
