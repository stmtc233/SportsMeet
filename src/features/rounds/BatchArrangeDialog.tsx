import { useState } from "react";
import { StButton, StDialog, StMuted, StSegmented, StSwitchItem, StTag } from "../../design";
import type { BatchArrangePreview, BatchArrangeRequest, RoundRow } from "../../mygo";
import { useMeetSettings } from "../meets/data";
import { RoundsController } from "./controller";

/**
 * 自动编排 of several rounds at once: the same options for all, each event
 * at its own heat size. Go previews it; 「编排」 saves exactly that preview.
 */
export function BatchArrangeDialog({ meetId, rounds, onClose, onDone }: { meetId: string; rounds: RoundRow[]; onClose: () => void; onDone: () => void }) {
  const { data: preferences } = useMeetSettings(meetId);
  const [order, setOrder] = useState("seed");
  const [lanes, setLanes] = useState("center");
  const [separate, setSeparate] = useState<boolean>();
  const [balance, setBalance] = useState<boolean>();
  const [preview, setPreview] = useState<{ request: BatchArrangeRequest; result: BatchArrangePreview }>();
  const [busy, setBusy] = useState(false);
  const separateUnits = separate ?? preferences?.separateUnits ?? true;
  const balanced = balance ?? preferences?.balance ?? true;
  const anyAdvance = rounds.some((r) => !r.first);
  // Changing an option drops the preview: what is saved is what was shown.
  const change = <T,>(set: (v: T) => void) => (v: T) => { set(v); setPreview(undefined); };

  const run = async () => {
    setBusy(true);
    if (preview) {
      const ok = await RoundsController.arrange(preview.request, preview.result.previewHash);
      setBusy(false);
      if (ok) onDone();
      else setPreview(undefined);
      return;
    }
    // The seed is made here, so the saved draw is the one previewed.
    const request: BatchArrangeRequest = { meetId, rounds: rounds.map((r) => ({ eventId: r.eventId, roundId: r.roundId })), order, lanes,
      separateUnits, balance: balanced, seed: Math.floor(Math.random() * 2 ** 31) };
    const result = await RoundsController.preview(request);
    setBusy(false);
    if (result) setPreview({ request, result });
  };
  const result = preview?.result;
  const ready = result && result.errors.length === 0;
  const count = result?.rounds.filter((r) => !r.error).length ?? 0;
  return (
    <StDialog open title={`自动编排 ${rounds.length} 个轮次`} onClose={onClose} onSubmit={run}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          {preview && <StButton onClick={() => setPreview(undefined)} disabled={busy}>重新预览</StButton>}
          <StButton variant="primary" type="submit" loading={busy} disabled={!!result && !ready}>{preview ? `编排 ${count} 个轮次` : "预览"}</StButton>
        </>
      }>
      {!preview ? (
        <>
          <StSegmented label="排列顺序" value={order} onChange={change(setOrder)}
            options={[{ value: "seed", label: anyAdvance ? "按成绩" : "按报名成绩" }, { value: "random", label: "随机" }]} />
          <StSegmented label="位置分配" value={lanes} onChange={change(setLanes)}
            options={[{ value: "center", label: "好成绩排中间道" }, { value: "sequential", label: "依次" }, { value: "random", label: "随机" }]} />
          <div>
            <StSwitchItem title="同单位分散" subtitle="同单位尽量不同组" checked={separateUnits} onChange={change(setSeparate)} />
            <StSwitchItem title="均衡人数" checked={balanced} onChange={change(setBalance)} />
          </div>
        </>
      ) : (
        <div className="batch-preview">
          <StMuted>共 {result!.heats} 组 · {result!.people} 人，已有的组不动</StMuted>
          <ul className="batch-preview__list" aria-label="编排预览">
            {result!.rounds.map((r) => (
              <li key={r.roundId} className="batch-preview__row" data-skipped={r.error ? "" : undefined}>
                <span className="batch-preview__name">{r.text}</span>
                <StMuted>{r.source}</StMuted>
                {r.error ? <StTag tone="warning">{r.error}</StTag> : <span className="batch-preview__count">{r.heats} 组 · {r.people} 人</span>}
              </li>
            ))}
          </ul>
          {result!.errors.map((e) => <StTag key={e} tone="danger">{e}</StTag>)}
        </div>
      )}
    </StDialog>
  );
}
