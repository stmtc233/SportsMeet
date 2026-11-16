import { useEffect, useRef, useState } from "react";
import { IconDown, IconUp, StButton, StCheckbox, StDateField, StDialog, StEmpty, StIconButton, StNotice, StRow, StSection, StSegmented, StTable, StTextField, StTimeField, iconStroke } from "../../design";
import { joinScheduled, splitScheduled } from "../../lib/wallTime";
import type { RoundScheduleChange, ScheduleChange, SchedulePreview, ScheduleRequest, ScheduleSkip } from "../../mygo";
import { ConflictList } from "./ConflictList";
import { moveItem, toggleItem } from "./scheduleOrder";
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
  roundAt?: string;
}

/**
 * Schedules round starts before qualifiers are known, or unscored heats.
 * Multi-round mode defaults to round starts and keeps the selected order.
 * Go computes times, conflicts and confirmation. A changed version drops
 * the preview.
 */
export function ScheduleDialog({ options, multi = false, day, version, onClose }: {
  options: ScheduleOption[];
  multi?: boolean;
  day: string;
  version: unknown;
  onClose: () => void;
}) {
  const [mode, setMode] = useState<"rounds" | "heats">(multi ? "rounds" : "heats");
  const seed = options.find((o) => multi ? o.roundAt : o.firstAt) ?? options[0];
  const first = splitScheduled((multi ? seed?.roundAt : seed?.firstAt) ?? "");
  const [date, setDate] = useState(first.date || day);
  const [time, setTime] = useState(first.time || "09:00");
  const heatSeed = options.find((o) => o.firstAt) ?? options[0];
  const [intervals, setIntervals] = useState({ rounds: "30", heats: String(heatSeed?.minutes ?? 5) });
  const interval = intervals[mode];
  const setInterval = (value: string) => setIntervals((old) => ({ ...old, [mode]: value }));
  const [gap, setGap] = useState("0");
  // The rounds to schedule, in order: picking appends, 上移 / 下移 reorder.
  const [order, setOrder] = useState<string[]>(() => (multi ? [] : options.slice(0, 1).map((o) => o.roundId)));
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
    setOrder((old) => toggleItem(old, roundId));
  };
  const move = (roundId: string, delta: -1 | 1) => {
    invalidate();
    setOrder((old) => moveItem(old, roundId, delta));
  };
  const byId = new Map(options.map((o) => [o.roundId, o]));
  const chosen = order.flatMap((id) => byId.get(id) ?? []);
  const only = !multi && chosen.length === 1 ? chosen[0] : undefined;
  const request: ScheduleRequest = only
    ? { eventId: only.eventId, roundId: only.roundId, startAt: joinScheduled(date, time), intervalMinutes: Number(interval) }
    : { mode, rounds: chosen.map((o) => ({ eventId: o.eventId, roundId: o.roundId })), startAt: joinScheduled(date, time), intervalMinutes: Number(interval), roundGapMinutes: mode === "heats" ? Number(gap) : 0 };
  const ready = preview !== null && preview.errors.length === 0;
  const changeCount = preview ? preview.changes.length + preview.roundChanges.length : 0;
  const run = async () => {
    if (busy || chosen.length === 0) return;
    const mine = ++epoch.current;
    setBusy(true);
    setNotice("");
    if (ready) {
      if (changeCount === 0) { setBusy(false); return; }
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
    <StDialog open title="排时间" size={multi ? "large" : undefined} onClose={close} onSubmit={run}
      actions={<>
        <StButton disabled={busy} onClick={close}>取消</StButton>
        <StButton variant="primary" type="submit" loading={busy} disabled={chosen.length === 0 || (ready && changeCount === 0)} reserveWidthFor={["预览", "保存"]}>{ready ? "保存" : "预览"}</StButton>
      </>}>
      {multi && <StSegmented aria-label="排时间方式" value={mode} onChange={(v) => { invalidate(); setMode(v); }} options={[
        { value: "rounds", label: "轮次开始时间", disabled: busy }, { value: "heats", label: "逐组排时间", disabled: busy },
      ]} />}
      <StRow>
        <StDateField label={mode === "rounds" ? "首轮日期" : "首组日期"} value={date} disabled={busy} onChange={(v) => change(setDate, v)} />
        <StTimeField label={mode === "rounds" ? "首轮时间" : "首组时间"} value={time} disabled={busy} onChange={(v) => change(setTime, v)} />
        <StTextField label="间隔" inputMode="numeric" width="narrow" suffix="分钟" value={interval} disabled={busy} onChange={(v) => change(setInterval, v)} />
        {multi && mode === "heats" && <StTextField label="轮次间隔" inputMode="numeric" width="narrow" suffix="分钟" value={gap} disabled={busy} onChange={(v) => change(setGap, v)} />}
      </StRow>
      {multi && <StSection title={`轮次 ${chosen.length}/${options.length}`}>
        <div className="schedule-rounds">
          {options.map((o) => <StCheckbox key={o.roundId} checked={order.includes(o.roundId)} disabled={busy} onChange={() => toggle(o.roundId)} label={o.label} />)}
          {options.length === 0 && <StEmpty>没有轮次</StEmpty>}
        </div>
      </StSection>}
      {multi && chosen.length > 1 && <StSection title="先后顺序" padded={false}>
        <ol className="schedule-order" aria-label="先后顺序">
          {chosen.map((o, i) => (
            <li key={o.roundId} className="schedule-order__row">
              <span className="schedule-order__index">{i + 1}</span>
              <span className="schedule-order__label">{o.label}</span>
              <StIconButton size="small" label={`上移「${o.label}」`} disabled={busy || i === 0} onClick={() => move(o.roundId, -1)}><IconUp size={16} strokeWidth={iconStroke} /></StIconButton>
              <StIconButton size="small" label={`下移「${o.label}」`} disabled={busy || i === chosen.length - 1} onClick={() => move(o.roundId, 1)}><IconDown size={16} strokeWidth={iconStroke} /></StIconButton>
            </li>
          ))}
        </ol>
      </StSection>}
      {notice && <StNotice tone="warning">{notice}</StNotice>}
      {preview?.errors.map((error) => <StNotice key={error} tone="danger">{error}</StNotice>)}
      {ready && preview.warnings.map((warning) => <StNotice key={warning} tone="warning">{warning}</StNotice>)}
      {ready && mode === "rounds" && <StSection title={`修改 ${preview.roundChanges.length} 个轮次`}>
        <div className="preview-table schedule-table">
          <StTable<RoundScheduleChange> dense fill rows={preview.roundChanges} rowKey={(row) => row.roundId}
            empty={<StEmpty>没有需要更改的时间</StEmpty>} columns={[
              { key: "round", title: "轮次", width: "2fr", render: (row) => <span title={row.round}>{row.round}</span> },
              { key: "before", title: "原安排", width: "2fr", render: (row) => row.beforeText },
              { key: "after", title: "新安排", width: "2fr", render: (row) => row.afterText },
            ]} />
        </div>
      </StSection>}
      {ready && mode === "heats" && <StSection title={`修改 ${preview.changes.length} 组`}>
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
      {ready && preview.conflicts.length > 0 && <StSection title={`冲突 ${preview.conflicts.length} 处`} padded={false}>
        <ConflictList conflicts={preview.conflicts} />
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

