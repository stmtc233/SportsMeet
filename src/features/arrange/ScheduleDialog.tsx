import { useEffect, useRef, useState } from "react";
import { StButton, StCheckbox, StDateField, StDialog, StEmpty, StNotice, StRow, StSection, StTable, StTextField, StTimeField } from "../../design";
import { joinScheduled, splitScheduled } from "../../lib/wallTime";
import type { ConflictView, ScheduleChange, SchedulePreview, ScheduleRequest, ScheduleSkip } from "../../mygo";
import { ScheduleController } from "./scheduleController";

/** A round the dialog can schedule; firstAt and minutes seed the fields. */
export interface ScheduleOption {
  eventId: string;
  roundId: string;
  /** The event and round as listed: 「男子100m 决赛」. */
  label: string;
  /** The effective time of its first heat without results; "" = none. */
  firstAt: string;
  minutes: number | null;
}

/**
 * Schedules the unscored heats of one round, or of several rounds one after
 * another (multi: the rounds are picked here, in the listed order, with a
 * gap between rounds). The plan, its conflicts and the confirmed write all
 * come from Go, including all time arithmetic. version is the data the
 * options were read from: when it changes, the preview is dropped.
 */
export function ScheduleDialog({ options, multi = false, day, version, onClose }: {
  options: ScheduleOption[];
  multi?: boolean;
  day: string;
  version: unknown;
  onClose: () => void;
}) {
  const seed = options.find((o) => o.firstAt) ?? options[0];
  const first = splitScheduled(seed?.firstAt ?? "");
  const [date, setDate] = useState(first.date || day);
  const [time, setTime] = useState(first.time || "09:00");
  const [interval, setInterval] = useState(String(seed?.minutes ?? 5));
  const [gap, setGap] = useState("0");
  const [picked, setPicked] = useState<Set<string>>(() => new Set(multi ? [] : options.slice(0, 1).map((o) => o.roundId)));
  const [preview, setPreview] = useState<SchedulePreview | null>(null);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const epoch = useRef(0);
  const saving = useRef(false);
  const seen = useRef(version);
  useEffect(() => () => { epoch.current++; }, []);
  useEffect(() => {
    if (seen.current === version) return;
    seen.current = version;
    if (saving.current) return;
    epoch.current++;
    setBusy(false);
    setPreview(null);
    setNotice("数据已变化，请重新预览");
  }, [version]);
  const invalidate = () => {
    epoch.current++;
    setPreview(null);
    setNotice("");
  };
  const change = (set: (value: string) => void, value: string) => {
    invalidate();
    set(value);
  };
  const toggle = (roundId: string) => {
    invalidate();
    setPicked((old) => {
      const next = new Set(old);
      if (!next.delete(roundId)) next.add(roundId);
      return next;
    });
  };
  const chosen = options.filter((o) => picked.has(o.roundId));
  const only = !multi && chosen.length === 1 ? chosen[0] : undefined;
  const request: ScheduleRequest = only
    ? { eventId: only.eventId, roundId: only.roundId, startAt: joinScheduled(date, time), intervalMinutes: Number(interval) }
    : { rounds: chosen.map((o) => ({ eventId: o.eventId, roundId: o.roundId })), startAt: joinScheduled(date, time), intervalMinutes: Number(interval), roundGapMinutes: Number(gap) };
  const ready = preview !== null && preview.errors.length === 0;
  const run = async () => {
    if (busy || chosen.length === 0) return;
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
    <StDialog open title={multi ? "连排轮次" : "批量排时间"} size={multi ? "large" : undefined} onClose={close} onSubmit={run}
      actions={<>
        <StButton disabled={busy} onClick={close}>取消</StButton>
        <StButton variant="primary" type="submit" loading={busy} disabled={chosen.length === 0 || (ready && preview.changes.length === 0)} reserveWidthFor={["预览", "保存"]}>{ready ? "保存" : "预览"}</StButton>
      </>}>
      <StRow>
        <StDateField label="首组日期" value={date} disabled={busy} onChange={(v) => change(setDate, v)} />
        <StTimeField label="首组时间" value={time} disabled={busy} onChange={(v) => change(setTime, v)} />
        <StTextField label="间隔" inputMode="numeric" width="narrow" suffix="分钟" value={interval} disabled={busy} onChange={(v) => change(setInterval, v)} />
        {multi && <StTextField label="轮次间隔" inputMode="numeric" width="narrow" suffix="分钟" value={gap} disabled={busy} onChange={(v) => change(setGap, v)} />}
      </StRow>
      {multi && <StSection title={`轮次 ${chosen.length}/${options.length}`}>
        <div className="schedule-rounds">
          {options.map((o) => <StCheckbox key={o.roundId} checked={picked.has(o.roundId)} disabled={busy} onChange={() => toggle(o.roundId)} label={o.label} />)}
          {options.length === 0 && <StEmpty>没有轮次</StEmpty>}
        </div>
      </StSection>}
      {notice && <StNotice tone="warning">{notice}</StNotice>}
      {preview?.errors.map((error) => <StNotice key={error} tone="danger">{error}</StNotice>)}
      {ready && preview.warnings.map((warning) => <StNotice key={warning} tone="warning">{warning}</StNotice>)}
      {ready && <StSection title={`修改 ${preview.changes.length} 组`}>
        <div className="preview-table schedule-table">
          <StTable<ScheduleChange> dense fill rows={preview.changes} rowKey={(row) => row.heatId}
            empty={<StEmpty>没有需要更改的时间</StEmpty>}
            columns={[
              ...(multi ? [{ key: "round", title: "轮次", width: "2fr", render: (row: ScheduleChange) => <span title={row.round}>{row.round}</span> }] : []),
              { key: "group", title: "组", render: (row) => <span title={row.label}>{row.label}</span> },
              { key: "before", title: "原安排", width: "2fr", render: (row) => <span title={row.beforeText}>{row.beforeText}</span> },
              { key: "after", title: "新安排", width: "2fr", render: (row) => <span title={row.afterText}>{row.afterText}</span> },
            ]} />
        </div>
      </StSection>}
      {ready && preview.conflicts.length > 0 && <StSection title={`冲突 ${preview.conflicts.length} 处`}>
        <div className="preview-table schedule-table">
          <StTable<ConflictView> dense fill rows={preview.conflicts} rowKey={(row) => `${row.athleteId}|${row.first.heatId}|${row.second.heatId}`} columns={[
            { key: "athlete", title: "运动员", render: (row) => row.athleteName },
            { key: "text", title: "冲突", render: (row) => row.text },
            { key: "first", title: "组", width: "2fr", render: (row) => <HeatRefText value={row.first} /> },
            { key: "second", title: "组", width: "2fr", render: (row) => <HeatRefText value={row.second} /> },
          ]} />
        </div>
      </StSection>}
      {ready && preview.skipped.length > 0 && <StSection title={`跳过 ${preview.skipped.length} 组`}>
        <div className="preview-table schedule-table">
          <StTable<ScheduleSkip> dense fill rows={preview.skipped} rowKey={(row) => row.heatId} columns={[
            ...(multi ? [{ key: "round", title: "轮次", width: "2fr", render: (row: ScheduleSkip) => <span title={row.round}>{row.round}</span> }] : []),
            { key: "group", title: "组", render: (row) => <span title={row.label}>{row.label}</span> },
            { key: "reason", title: "原因", render: (row) => row.reason },
          ]} />
        </div>
      </StSection>}
    </StDialog>
  );
}

/** A heat named in a conflict: 「09:00 男子100m 决赛 第1组」. */
export function HeatRefText({ value }: { value: ConflictView["first"] }) {
  const text = `${value.timeText} ${value.eventName} ${value.roundLabel} ${value.heatLabel}`;
  return <span title={text}>{text}</span>;
}
