import { useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { IconTime, StButton, StIconButton, StMarkField, StMuted, StSection, StSelect, StTag, StChanged, iconStroke } from "../../design";
import { HeatStarted } from "../event/HeatStarted";
import { HeatTime } from "../event/HeatTime";
import { statusLabels } from "../../lib/labels";
import type { EntryView, EventDetail, HeatView, ResultInput, Status } from "../../mygo";
import { ResultsController } from "./controller";
import { mergeRows, sameRow, toRow, toRows, type Row, type Rows } from "./rows";
import { StartedDialog } from "./StartedDialog";
import { HeightResults } from "./HeightResults";

export interface HeatResultsProps { detail: EventDetail; heat: HeatView; entries: Map<string, EntryView>; onDirty: (heatId: string, dirty: boolean) => void }

export function HeatResults(props: HeatResultsProps) {
  return props.detail.event.kind === "height" ? <HeightResults {...props} /> : <StandardHeatResults {...props} />;
}

const isAttemptWord = (t: string) => /^(x|×|-|—|犯规|免跳|免投|免)$/i.test(t);

// Enter moves to the same column of the next row.
function nextRow(e: KeyboardEvent<HTMLInputElement>) {
  if (e.key !== "Enter") return;
  e.preventDefault();
  const name = e.currentTarget.name;
  const [col, row] = name.split(":");
  const fields = [...(e.currentTarget.closest(".result-grid")?.querySelectorAll<HTMLInputElement>(`input[name^="${col}:"]`) ?? [])];
  const next = fields.filter((input) => !input.disabled && (e.shiftKey ? Number(input.name.split(":")[1]) < Number(row) : Number(input.name.split(":")[1]) > Number(row)));
  const target = e.shiftKey ? next.at(-1) : next[0];
  e.currentTarget.blur();
  target?.focus();
}

/** One heat's results: typed per position, saved together. Places come back from Go. */
function StandardHeatResults({ detail, heat, entries, onDirty }: HeatResultsProps) {
  const kind = detail.event.kind;
  const timed = detail.rules.timed;
  const attempts = detail.rules.multipleAttempts;
  const direct = !attempts;
  const initial = useMemo(() => toRows(heat.slots, attempts ? detail.event.attemptCount : 0), [heat, attempts, detail.event.attemptCount]);
  const [rows, setRows] = useState<Rows>(initial);
  const [busy, setBusy] = useState(false);
  const [editStart, setEditStart] = useState(false);
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

  const set = (pos: number, patch: Partial<Row>) => setRows((old) => ({ ...old, [pos]: { ...old[pos]!, ...patch } }));
  const discard = () => {
    setRows(initial);
  };
  const save = async () => {
    const sent = rows;
    const loaded = base.current;
    const inputs: ResultInput[] = heat.slots.filter((s) => !sameRow(rows[s.position], initial[s.position])).map((s) => {
      const r = rows[s.position]!;
      return { position: s.position, status: r.status, mark: direct ? r.mark : "", attempts: attempts ? r.attempts : [], note: s.result?.note ?? "",
        bars: [] };
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

  const inputCols = direct ? 1 : detail.event.attemptCount;
  // Fixed and fr tracks only, so every row lays out the same columns.
  const style = { "--result-cols": `48px minmax(96px, 1fr) minmax(72px, 1fr) repeat(${inputCols}, minmax(64px, 88px)) 96px ${direct ? "" : "80px "}56px 40px` } as CSSProperties;
  const statuses: Status[] = timed ? ["DNS", "DNF", "DQ"] : kind === "count" || kind === "score" ? ["DNS", "DNF", "DQ", "NM"] : ["DNS", "DQ", "NM"];
  return (
    <StSection padded={false}
      title={
        <span className="heat-title">
          <span>{heat.label}</span>
          <HeatTime text={heat.scheduledText} inherited={heat.scheduleInherited} />
          <HeatStarted text={heat.startedText} delta={heat.startDeltaText} />
        </span>
      }
      actions={
        <>
          {heat.resultsUpdatedText && <StMuted>修改于 {heat.resultsUpdatedText}</StMuted>}
          <StIconButton size="small" label="开赛时间" onClick={() => setEditStart(true)}><IconTime size={16} strokeWidth={iconStroke} /></StIconButton>
          <StButton disabled={!dirty} onClick={discard}>放弃</StButton>
          <StButton variant="primary" disabled={!dirty} loading={busy} onClick={save}>保存</StButton>
        </>
      }>
      <div className="result-grid" style={style} role="table">
        <div className="result-rows">
          <div className="result-row result-row--head" role="row">
            <span className="head pos" role="columnheader">{detail.rules.positionLabel}</span>
            <span className="head" role="columnheader">{detail.rules.team ? "队伍" : "姓名"}</span>
            <span className="head" role="columnheader">单位</span>
            {direct && <span className="head num" role="columnheader">{detail.rules.markLabel}</span>}
            {attempts && Array.from({ length: inputCols }, (_, i) => <span key={i} className="head num" role="columnheader">{detail.rules.attemptLabel} {i + 1}</span>)}
            <span className="head" role="columnheader">状态</span>
            {!direct && <span className="head num" role="columnheader">{detail.rules.markLabel}</span>}
            <span className="head num" role="columnheader">名次</span>
            <span className="head" role="columnheader" />
          </div>
          {heat.slots.map((s, i) => {
            const row = rows[s.position] ?? toRow(s, 0);
            const e = entries.get(s.competitorId);
            const off = row.status !== "";
            return (
              <div key={s.position} className="result-row" role="row" data-off={off || undefined} data-dirty={!sameRow(row, initial[s.position]) || undefined}>
                <span className="pos" role="cell">{s.position}</span>
                <span className="name" role="cell" title={e?.name}>{e?.name}</span>
                <span className="unit" role="cell" title={e?.unitName}>{e?.unitName}</span>
                {direct && <StMarkField event={detail.event} kind={kind} name={`m:${i}`} value={row.mark} disabled={off} width="fill" aria-label={detail.rules.markLabel} onChange={(mark) => set(s.position, { mark })} onKeyDown={nextRow} />}
                {attempts && row.attempts.slice(0, inputCols).map((a, j) => (
                  <StMarkField key={j} event={detail.event} kind={kind} name={`a${j}:${i}`} value={a} disabled={off} width="fill" accept={isAttemptWord} aria-label={`${detail.rules.attemptLabel} ${j + 1}`} onKeyDown={nextRow}
                    onChange={(v) => set(s.position, { attempts: row.attempts.map((old, k) => (k === j ? v : old)) })} />
                ))}
                <StSelect size="small" width="fill" value={row.status} placeholder="正常" onChange={(status) => set(s.position, { status })}
                  options={[{ value: "", label: "正常" }, ...statuses.map((v) => ({ value: v as string, label: statusLabels[v] }))]} />
                {!direct && <span className="result-mark" role="cell"><StChanged value={s.result?.markText}>{s.result?.markText}</StChanged></span>}
                <span className="rank" role="cell"><StChanged value={s.result?.rank}>{s.result?.rank ?? ""}</StChanged></span>
                <span role="cell"><StChanged value={s.result?.qualify}>{s.result?.qualify && <StTag tone="success">{s.result.qualify}</StTag>}</StChanged></span>
              </div>
            );
          })}
        </div>
      </div>
      {editStart && <StartedDialog heat={heat} day={detail.day} onClose={() => setEditStart(false)} />}
    </StSection>
  );
}
