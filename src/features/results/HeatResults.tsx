import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { StButton, StMarkField, StRow, StSection, StSelect, StTag, StTextField } from "../../design";
import { statusLabels } from "../../lib/labels";
import type { EntryView, EventDetail, HeatView, ResultInput, Status } from "../../mygo";
import { ResultsController } from "./controller";
import { mergeRows, sameRow, toRow, toRows, type Row, type Rows } from "./rows";

const isAttemptWord = (t: string) => /^(x|×|-|—|犯规|免跳|免投|免)$/i.test(t);
const isTrials = (t: string) => /^[oOxX0×\-—]{1,3}$/.test(t);

// Enter moves to the same column of the next row.
function nextRow(e: KeyboardEvent<HTMLInputElement>) {
  if (e.key !== "Enter") return;
  e.preventDefault();
  const name = e.currentTarget.name;
  const [col, row] = name.split(":");
  const next = e.currentTarget.closest(".result-grid")?.querySelector<HTMLInputElement>(`input[name="${col}:${Number(row) + 1}"]`);
  (next ?? e.currentTarget).blur();
  next?.focus();
}

/** One heat's results: typed per position, saved together. Places come back from Go. */
export function HeatResults({ detail, heat, entries, onDirty }: { detail: EventDetail; heat: HeatView; entries: Map<string, EntryView>; onDirty: (heatId: string, dirty: boolean) => void }) {
  const kind = detail.event.kind;
  const timed = kind === "track" || kind === "relay";
  const initial = useMemo(() => toRows(heat.slots, kind === "distance" ? detail.event.attemptCount : 0), [heat, kind, detail.event.attemptCount]);
  const [rows, setRows] = useState<Rows>(initial);
  const [extra, setExtra] = useState<string[]>([]);
  const [height, setHeight] = useState("");
  const [busy, setBusy] = useState(false);
  // base is what rows were typed over; accepted is what a save sent and
  // the next reload has not shown yet.
  const base = useRef(initial);
  const accepted = useRef<Rows | undefined>(undefined);
  const dirty = heat.slots.some((s) => !sameRow(rows[s.position], initial[s.position]));
  useEffect(() => {
    const from = base.current;
    const sent = accepted.current;
    if (from === initial) return;
    base.current = initial;
    accepted.current = undefined;
    setRows((old) => mergeRows(from, initial, old, sent));
  }, [initial]);
  useEffect(() => {
    onDirty(heat.id, dirty);
    return () => onDirty(heat.id, false);
  }, [heat.id, dirty, onDirty]);

  const heights = useMemo(() => {
    const all = new Set<string>(extra);
    for (const s of heat.slots) for (const b of s.result?.bars ?? []) all.add(b.heightText);
    return [...all].sort((a, b) => Number(a) - Number(b));
  }, [heat, extra]);
  const set = (pos: number, patch: Partial<Row>) => setRows((old) => ({ ...old, [pos]: { ...old[pos]!, ...patch } }));
  const addHeight = async () => {
    const tidy = await ResultsController.normalize(kind, height);
    if (typeof tidy !== "string") return;
    setExtra((list) => (list.includes(tidy) ? list : [...list, tidy]));
    setHeight("");
  };
  const discard = () => {
    setRows(initial);
    setExtra([]);
  };
  const save = async () => {
    const sent = rows;
    const loaded = base.current;
    const inputs: ResultInput[] = heat.slots.filter((s) => !sameRow(rows[s.position], initial[s.position])).map((s) => {
      const r = rows[s.position]!;
      return { position: s.position, status: r.status, mark: timed ? r.mark : "", attempts: kind === "distance" ? r.attempts : [], note: s.result?.note ?? "",
        bars: kind === "height" ? heights.map((h) => ({ height: h, trials: r.bars[h] ?? "" })) : [] };
    });
    setBusy(true);
    const saved = await ResultsController.save(heat.id, inputs);
    setBusy(false);
    if (saved === undefined) return;
    if (base.current === loaded) {
      accepted.current = sent; // the reload this save causes shows the stored text
    } else {
      const now = base.current; // that reload came first
      setRows((old) => mergeRows(now, now, old, sent));
    }
  };

  const inputCols = timed ? 1 : kind === "distance" ? detail.event.attemptCount : heights.length;
  const style: CSSProperties = { gridTemplateColumns: `36px minmax(88px, 1.2fr) minmax(72px, 1fr) repeat(${inputCols}, minmax(64px, 88px)) 112px 72px 48px 40px` };
  const statuses: Status[] = timed ? ["DNS", "DNF", "DQ"] : ["DNS", "DQ", "NM"];
  return (
    <StSection padded={false}
      title={
        <>
          {heat.label}
          {heat.scheduledAt && <StTag>{heat.scheduledAt.split("T")[1]}</StTag>}
        </>
      }
      actions={
        <>
          {kind === "height" && (
            <StRow wrap={false}>
              <StTextField value={height} onChange={setHeight} placeholder="高度" width="narrow" suffix="m" aria-label="高度" />
              <StButton disabled={height.trim() === ""} onClick={addHeight}>添加高度</StButton>
            </StRow>
          )}
          <StButton disabled={!dirty} onClick={discard}>放弃</StButton>
          <StButton variant="primary" disabled={!dirty} loading={busy} onClick={save}>保存</StButton>
        </>
      }>
      <div className="result-grid" style={style}>
        <span className="head num">{detail.rules.positionLabel}</span>
        <span className="head">{kind === "relay" ? "队伍" : "姓名"}</span>
        <span className="head">单位</span>
        {timed && <span className="head num">成绩</span>}
        {kind === "distance" && Array.from({ length: inputCols }, (_, i) => <span key={i} className="head num">{detail.rules.attemptLabel} {i + 1}</span>)}
        {kind === "height" && heights.map((h) => <span key={h} className="head num">{h}</span>)}
        <span className="head">状态</span>
        <span className="head num">{timed ? "" : "成绩"}</span>
        <span className="head num">名次</span>
        <span className="head" />
        {heat.slots.map((s, i) => {
          const row = rows[s.position] ?? toRow(s, 0);
          const e = entries.get(s.competitorId);
          const off = row.status !== "";
          return [
            <span key={`p${s.position}`} className="num">{s.position}</span>,
            <span key={`n${s.position}`} className="name">{e?.name}</span>,
            <span key={`u${s.position}`} className="name st-muted">{e?.unitName}</span>,
            timed && <StMarkField key={`m${s.position}`} kind={kind} name={`m:${i}`} value={row.mark} disabled={off} width="fill" aria-label="成绩" onChange={(mark) => set(s.position, { mark })} onKeyDown={nextRow} />,
            kind === "distance" && row.attempts.slice(0, inputCols).map((a, j) => (
              <StMarkField key={`a${s.position}-${j}`} kind={kind} name={`a${j}:${i}`} value={a} disabled={off} width="fill" accept={isAttemptWord} aria-label={`${detail.rules.attemptLabel} ${j + 1}`} onKeyDown={nextRow}
                onChange={(v) => set(s.position, { attempts: row.attempts.map((old, k) => (k === j ? v : old)) })} />
            )),
            kind === "height" && heights.map((h, j) => (
              <StMarkField key={`b${s.position}-${h}`} kind={kind} name={`b${j}:${i}`} value={row.bars[h] ?? ""} disabled={off} width="fill" accept={isTrials} placeholder="OX-" aria-label={h} onKeyDown={nextRow}
                onChange={(v) => set(s.position, { bars: { ...row.bars, [h]: v.toUpperCase() } })} />
            )),
            <StSelect key={`s${s.position}`} size="small" width="fill" value={row.status} placeholder="正常" onChange={(status) => set(s.position, { status })}
              options={[{ value: "", label: "正常" }, ...statuses.map((v) => ({ value: v as string, label: statusLabels[v] }))]} />,
            <span key={`r${s.position}`} className="result-mark">{timed ? "" : s.result?.markText}</span>,
            <span key={`k${s.position}`} className="num">{s.result?.rank ?? ""}</span>,
            <span key={`q${s.position}`}>{s.result?.qualify && <StTag tone="success">{s.result.qualify}</StTag>}</span>,
          ];
        })}
      </div>
    </StSection>
  );
}
