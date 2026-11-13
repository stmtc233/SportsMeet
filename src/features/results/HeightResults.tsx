import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type MouseEvent } from "react";
import { IconAdd, IconCheck, IconMore, IconTime, IconUndo, StButton, StDialog, StIconButton, StMenu, StMuted, StSection, StTag, StTextField, StTrialsField, iconStroke } from "../../design";
import { statusLabels } from "../../lib/labels";
import { HeatStarted } from "../event/HeatStarted";
import { HeatTime } from "../event/HeatTime";
import { ResultsController, useHeightPreview } from "./controller";
import type { HeatResultsProps } from "./HeatResults";
import { heightColumns, heightInputs, trialSymbol } from "./height";
import { mergeRows, sameRow, toRows, type Rows } from "./rows";
import { StartedDialog } from "./StartedDialog";

const statuses = ["DNS", "DQ", "NM"] as const;

export function HeightResults({ detail, heat, entries, onDirty }: HeatResultsProps) {
  const initial = useMemo(() => toRows(heat.slots, 0), [heat]);
  const [rows, setRows] = useState(initial);
  const [extra, setExtra] = useState<string[]>([]);
  const [removed, setRemoved] = useState<string[]>([]);
  const [height, setHeight] = useState("");
  const [heightError, setHeightError] = useState("");
  const [selected, setSelected] = useState<{ position: number; height: string }>();
  const [typing, setTyping] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [adding, setAdding] = useState(false);
  const [editStart, setEditStart] = useState(false);
  const [remove, setRemove] = useState<string>();
  const [finish, setFinish] = useState(false);
  const [renaming, setRenaming] = useState<string>();
  const [replacement, setReplacement] = useState("");
  const [renameError, setRenameError] = useState("");
  const base = useRef(initial);
  const accepted = useRef<Rows | undefined>(undefined);
  const grid = useRef<HTMLDivElement>(null);
  // A trial needs the Go preview of the cell it extends, so keys pressed
  // before it arrives wait here. After a new height the cursor moves to the
  // next trial once its preview arrives; recording a trial leaves it in place.
  const follow = useRef(false);
  const queue = useRef<string[]>([]);
  const heights = useMemo(() => heightColumns(heat, rows, extra, removed), [heat, rows, extra, removed]);
  const inputs = useMemo(() => heightInputs(heat, rows, heights), [heat, rows, heights]);
  const state = useHeightPreview(inputs);
  const preview = state?.preview;
  const rowPreviews = new Map(preview?.rows.map((row) => [row.position, row]));
  const next = preview?.next ?? undefined;
  const changed = heat.slots.some((slot) => !sameRow(rows[slot.position], initial[slot.position]));
  const dirty = changed || extra.length > 0 || removed.length > 0 || height.trim() !== "";
  const completed = heat.slots.length > 0 && heat.slots.every((slot) => slot.result && !slot.result.heightPending);
  const active = selected && heights.includes(selected.height) && rows[selected.position]?.status === "" ? selected : undefined;
  const activeSlot = heat.slots.find((slot) => slot.position === active?.position);
  const activeRow = active ? rowPreviews.get(active.position) : undefined;
  const activeCell = activeRow?.cells.find((cell) => cell.height === active?.height);
  const activeName = activeSlot ? entries.get(activeSlot.competitorId)?.name : "";
  const suggestion = preview?.nextHeight ?? "";

  useEffect(() => {
    const from = base.current;
    if (from === initial) return;
    base.current = initial;
    const sent = accepted.current;
    accepted.current = undefined;
    setRows((old) => mergeRows(from, initial, old, sent));
    const stored = new Set(heat.slots.flatMap((slot) => slot.result?.bars.map((bar) => bar.heightText) ?? []));
    setExtra((old) => old.filter((h) => !stored.has(h)));
    setRemoved((old) => old.filter((h) => stored.has(h)));
  }, [initial, heat]);
  useEffect(() => {
    onDirty(heat.id, dirty);
    return () => onDirty(heat.id, false);
  }, [heat.id, dirty, onDirty]);

  const cellInput = (position: number, h: string) => grid.current?.querySelector<HTMLInputElement>(`input[name="h${h}:${position}"]`);
  const setTrials = (position: number, h: string, trials: string) => setRows((old) => ({ ...old, [position]: { ...old[position]!, bars: { ...old[position]!.bars, [h]: trials } } }));
  const record = (symbol: string) => {
    if (!preview || follow.current) { queue.current.push(symbol); return; }
    if (!active || !activeCell) return;
    const allowed = symbol === "O" ? activeCell.canSuccess : symbol === "X" ? activeCell.canFailure : activeCell.canPass;
    if (!allowed) return;
    setTrials(active.position, active.height, activeCell.trials + symbol);
  };
  const undo = () => {
    if (!active) return;
    const trials = activeCell?.trials || (rows[active.position]?.bars[active.height] ?? "").trim();
    setTrials(active.position, active.height, trials.slice(0, -1));
    cellInput(active.position, active.height)?.focus();
  };
  useEffect(() => {
    if (!preview) return;
    if (follow.current) {
      follow.current = false;
      if (!next) return;
      cellInput(next.position, next.height)?.focus();
      // Focusing a new cell re-renders and comes back here; staying put does not.
      if (active?.position !== next.position || active.height !== next.height) return;
    }
    const symbol = queue.current.shift();
    if (symbol) record(symbol);
  });

  const move = (from: HTMLInputElement, key: string) => {
    const enabled = (list: NodeListOf<HTMLInputElement> | undefined) => [...(list ?? [])].filter((input) => !input.disabled);
    const [h] = from.name.slice(1).split(":");
    const fields = key === "ArrowUp" || key === "ArrowDown"
      ? enabled(grid.current?.querySelectorAll<HTMLInputElement>(`input[name^="h${h}:"]`))
      : enabled(from.closest(".result-row")?.querySelectorAll<HTMLInputElement>('input[name^="h"]'));
    const target = fields[fields.indexOf(from) + (key === "ArrowUp" || key === "ArrowLeft" ? -1 : 1)];
    target?.focus();
  };
  const keyDown = (event: KeyboardEvent<HTMLInputElement>, cell: string) => {
    if (event.nativeEvent.isComposing || event.metaKey || event.ctrlKey || event.altKey) return;
    const key = event.key;
    if (typing === cell) {
      if (key === "Enter" || key === "Escape") { event.preventDefault(); setTyping(undefined); }
      return;
    }
    const symbol = trialSymbol(key);
    if (symbol) { event.preventDefault(); record(symbol); }
    else if (key === "Backspace" || key === "Delete") { event.preventDefault(); undo(); }
    else if (key === "Enter" || key === "F2") { event.preventDefault(); follow.current = false; setTyping(cell); event.currentTarget.select(); }
    else if (key.startsWith("Arrow")) { event.preventDefault(); follow.current = false; move(event.currentTarget, key); }
  };

  const addHeight = async () => {
    const value = height.trim() || suggestion;
    if (adding || busy || !value) return;
    setAdding(true);
    const tidy = await ResultsController.height(value);
    setAdding(false);
    if (tidy === undefined) return;
    if (heights.includes(tidy)) { setHeightError("该高度已存在"); return; }
    setExtra((old) => old.includes(tidy) ? old : [...old, tidy]);
    setRemoved((old) => old.filter((h) => h !== tidy));
    setHeight(""); setHeightError("");
    follow.current = true;
  };
  const removeHeight = (h: string) => {
    setExtra((old) => old.filter((value) => value !== h));
    if (heat.slots.some((slot) => slot.result?.bars.some((bar) => bar.heightText === h))) setRemoved((old) => old.includes(h) ? old : [...old, h]);
    setRows((old) => Object.fromEntries(Object.entries(old).map(([pos, row]) => [pos, { ...row, bars: Object.fromEntries(Object.entries(row.bars).filter(([key]) => key !== h)) }])));
    setSelected((old) => old?.height === h ? undefined : old);
    setRemove(undefined);
  };
  const renameHeight = async () => {
    if (!renaming || adding) return;
    setAdding(true);
    const tidy = await ResultsController.height(replacement);
    setAdding(false);
    if (!tidy) return;
    if (tidy !== renaming && heights.includes(tidy)) { setRenameError("该高度已存在"); return; }
    if (tidy !== renaming) {
      const old = renaming;
      if (heat.slots.some((slot) => slot.result?.bars.some((bar) => bar.heightText === old))) setRemoved((values) => [...values, old]);
      setExtra((values) => [...values.filter((value) => value !== old), tidy]);
      setRemoved((values) => values.filter((value) => value !== tidy));
      setRows((values) => Object.fromEntries(Object.entries(values).map(([pos, row]) => [pos, { ...row, bars: Object.fromEntries(Object.entries(row.bars).map(([h, trials]) => [h === old ? tidy : h, trials])) }])));
      setSelected((value) => value?.height === old ? { ...value, height: tidy } : value);
    }
    setRenaming(undefined);
  };
  const discard = () => {
    setRows(initial); setExtra([]); setRemoved([]); setHeight(""); setHeightError(""); setSelected(undefined); setTyping(undefined);
    follow.current = false; queue.current = [];
  };
  const save = async (finished = false) => {
    if (!preview?.valid || busy) return;
    const sent = rows;
    const loaded = base.current;
    const all = heightInputs(heat, rows, heights, finished);
    const writes = finished ? all : all.filter((input) => !sameRow(rows[input.position], initial[input.position]));
    setBusy(true);
    const saved = await ResultsController.save(heat.id, writes);
    setBusy(false);
    if (saved === undefined) return;
    setFinish(false);
    if (finished) { setExtra([]); setHeight(""); setHeightError(""); }
    if (base.current === loaded) accepted.current = sent;
    else setRows((old) => mergeRows(base.current, base.current, old, sent));
  };
  const setStatus = (position: number, status: string) => setRows((old) => ({ ...old, [position]: { ...old[position]!, status } }));
  // repeat(0, …) is invalid and would void the whole template.
  const style = { "--result-cols": ["40px 152px", heights.length ? `repeat(${heights.length}, 72px)` : "", "128px 72px 48px 32px"].filter(Boolean).join(" ") } as CSSProperties;
  // Buttons beside the cell leave focus in it, so keys keep working after a click.
  const keep = (event: MouseEvent) => event.preventDefault();
  const attempt = activeCell?.state === "open" ? activeCell.trials.length + 1 : 0;

  return <StSection padded={false} title={<span className="heat-title"><span>{heat.label}</span><HeatTime text={heat.scheduledText} inherited={heat.scheduleInherited} /><HeatStarted text={heat.startedText} delta={heat.startDeltaText} /></span>}
    actions={<><StTag tone={completed && !changed ? "success" : "neutral"}>{completed && !changed ? "已结束" : "录入中"}</StTag><StIconButton size="small" label="开赛时间" disabled={busy} onClick={() => setEditStart(true)}><IconTime size={16} strokeWidth={iconStroke} /></StIconButton></>}>
    <div className="height-editor" role="group" aria-label="试跳操作">
      <div className="height-editor__subject">
        <strong>{active ? activeName : heights.length ? "选择试跳格" : "添加起跳高度"}</strong>
        {active && <StTag>{active.height} m</StTag>}
        {attempt > 0 && <StMuted>第 {attempt} 次</StMuted>}
        {activeRow?.markText && <StMuted>当前最好 {activeRow.markText}</StMuted>}
      </div>
    </div>
    <div className="result-grid height-grid" style={style} role="table" ref={grid} onMouseDown={() => { follow.current = false; }} aria-label={`${heat.label}高度成绩`}>
      <div className="result-rows">
        <div className="result-row result-row--head" role="row">
          <span className="head pos height-sticky-pos" role="columnheader">{detail.rules.positionLabel}</span><span className="head height-identity" role="columnheader">姓名／单位</span>
          {heights.map((h) => <span key={h} className="head height-heading" role="columnheader" aria-label={`${h} m`} data-active={active?.height === h || undefined}>
            <StMenu trigger={(p) => <StButton variant="ghost" size="small" aria-label={`高度 ${h}`} disabled={busy} {...p}>{h}</StButton>}
              items={[
                { label: "修改高度", onSelect: () => { setRenaming(h); setReplacement(h); setRenameError(""); } },
                { label: "移除高度", danger: true, onSelect: () => Object.values(rows).some((row) => row.bars[h]?.trim()) ? setRemove(h) : removeHeight(h) },
              ]} />
          </span>)}
          <span className="head height-add" role="columnheader">
            <StTextField aria-label="高度" width="fill" placeholder={suggestion || "起跳高度"} value={height} disabled={busy || adding} error={heightError}
              onChange={(value) => { setHeight(value); setHeightError(""); }} onKeyDown={(event) => { if (event.key === "Enter" && !event.nativeEvent.isComposing) { event.preventDefault(); void addHeight(); } }} />
            <StIconButton size="small" label="添加高度" disabled={busy || adding || !(height.trim() || suggestion)} onClick={addHeight}><IconAdd size={16} strokeWidth={iconStroke} /></StIconButton>
          </span>
          <span className="head num height-end height-end--mark" role="columnheader">当前最好</span><span className="head num height-end height-end--rank" role="columnheader">名次</span><span className="head height-end height-end--menu" role="columnheader" />
        </div>
        {heat.slots.map((slot) => {
          const row = rows[slot.position] ?? initial[slot.position]!;
          const entry = entries.get(slot.competitorId);
          const result = rowPreviews.get(slot.position);
          const errorId = `${heat.id}-height-error-${slot.position}`;
          const status = row.status as (typeof statuses)[number] | "";
          return <div key={slot.position} className="result-row" role="row" data-dirty={!sameRow(row, initial[slot.position]) || undefined} data-off={status !== "" || undefined}>
            <span className="pos height-sticky-pos" role="cell">{slot.position}</span>
            <span className="height-identity" role="cell">
              <span className="height-identity__name"><span className="name">{entry?.name}</span>{status && <StTag tone="warning">{statusLabels[status]}</StTag>}</span>
              <span className="unit">{[entry?.bib, entry?.unitName].filter(Boolean).join(" · ")}</span>
            </span>
            {heights.map((h) => {
              const cell = result?.cells.find((c) => c.height === h);
              const key = `${h}:${slot.position}`;
              return <span key={h} className="height-cell" role="cell" data-state={cell?.state}>
                <StTrialsField name={`h${key}`} aria-label={`${h} m试跳`} value={row.bars[h] ?? ""} readOnly={typing !== key} disabled={busy || status !== ""} invalid={!!cell?.error} aria-describedby={cell?.error ? errorId : undefined}
                  onFocus={() => setSelected({ position: slot.position, height: h })} onBlur={() => setTyping((old) => old === key ? undefined : old)}
                  onDoubleClick={(event) => { setTyping(key); event.currentTarget.select(); }}
                  onChange={(value) => setTrials(slot.position, h, value)} onKeyDown={(event) => keyDown(event, key)} />
              </span>;
            })}
            <span className="height-actions" role="cell">{active?.position === slot.position && <>
              <StIconButton size="small" variant="tonal" label="成功 O" disabled={busy || !activeCell?.canSuccess} onMouseDown={keep} onClick={() => record("O")}>O</StIconButton>
              <StIconButton size="small" variant="tonal" label="失败 X" disabled={busy || !activeCell?.canFailure} onMouseDown={keep} onClick={() => record("X")}>X</StIconButton>
              <StIconButton size="small" variant="tonal" label="免跳 -" disabled={busy || !activeCell?.canPass} onMouseDown={keep} onClick={() => record("-")}>-</StIconButton>
              <StIconButton size="small" label="撤销一次" disabled={busy || !row.bars[active.height]} onMouseDown={keep} onClick={undo}><IconUndo size={16} strokeWidth={iconStroke} /></StIconButton>
            </>}</span>
            <span className="result-mark height-end height-end--mark" role="cell">{preview ? result?.markText || "—" : "…"}</span>
            <span className="rank height-end height-end--rank" role="cell">{preview ? result?.rank ?? "" : ""}</span>
            <span className="height-end height-end--menu" role="cell">
              <StMenu trigger={(p) => <StIconButton size="small" label="状态" disabled={busy} {...p}><IconMore size={16} strokeWidth={iconStroke} /></StIconButton>}
                items={[
                  ...statuses.map((value) => ({ label: statusLabels[value], icon: status === value ? <IconCheck size={16} strokeWidth={iconStroke} /> : undefined, onSelect: () => setStatus(slot.position, value) })),
                  { label: "清除状态", separated: true, disabled: status === "", onSelect: () => setStatus(slot.position, "") },
                ]} />
            </span>
            {result?.error && <span className="height-row-error" id={errorId} role="alert">{result.error}</span>}
          </div>;
        })}
      </div>
    </div>
    <div className="height-footer">
      <StMuted>{state?.error || (heat.resultsUpdatedText ? `修改于 ${heat.resultsUpdatedText}` : "")}</StMuted>
      {state.error && <StButton size="small" onClick={state.retry}>重新校验</StButton>}
      <div className="height-footer__buttons"><StButton disabled={!dirty || busy} onClick={discard}>放弃</StButton><StButton disabled={!preview?.canFinish || completed && !changed || busy} onClick={() => setFinish(true)}>结束录入</StButton><StButton variant="primary" disabled={!changed || !preview?.valid || adding} loading={busy} onClick={() => save()}>保存</StButton></div>
    </div>
    <StDialog open={remove !== undefined} title="移除高度？" size="small" onClose={() => setRemove(undefined)} actions={<><StButton onClick={() => setRemove(undefined)}>取消</StButton><StButton variant="danger" onClick={() => removeHeight(remove!)}>移除</StButton></>}>将移除 {remove} m 的全部试跳记录，保存后生效。</StDialog>
    <StDialog open={renaming !== undefined} title="修改高度" size="small" onClose={() => { if (!adding) setRenaming(undefined); }} onSubmit={renameHeight} actions={<><StButton disabled={adding} onClick={() => setRenaming(undefined)}>取消</StButton><StButton variant="primary" disabled={!replacement.trim()} loading={adding} onClick={renameHeight}>确定</StButton></>}><StTextField label="高度" suffix="m" value={replacement} error={renameError} disabled={adding} onChange={(value) => { setReplacement(value); setRenameError(""); }} /><StMuted>该列试跳记录一并保留。</StMuted></StDialog>
    <StDialog open={finish} title="结束这一组录入？" size="small" onClose={() => { if (!busy) setFinish(false); }} actions={<><StButton disabled={busy} onClick={() => setFinish(false)}>取消</StButton><StButton variant="primary" loading={busy} disabled={!preview?.canFinish} onClick={() => save(true)}>结束录入</StButton></>}>将保存当前记录并确认本组成绩。全部组结束后计算晋级。</StDialog>
    {editStart && <StartedDialog heat={heat} day={detail.day} onClose={() => setEditStart(false)} />}
  </StSection>;
}
