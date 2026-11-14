import { useEffect, useMemo, useRef, useState } from "react";
import { useLeaveGuard } from "../../app/nav";
import { IconAdd, IconAuto, IconMore, IconRedo, IconTime, IconUndo, StButton, StConfirmButton, StDialog, StEmpty, StIconButton, StMenu, StEnterScope, StSegmented, StTextField, iconStroke, toast } from "../../design";
import { errorText, isStale } from "../../lib/errors";
import { useScrollMemory, useViewState } from "../../lib/viewState";
import type { EntryView, EventDetail } from "../../mygo";
import { AddEntriesDialog } from "../event/AddEntriesDialog";
import { AutoDialog } from "./AutoDialog";
import { ArrangeController, useDraftConflicts } from "./controller";
import { addHeat, appendPlanned, deleteHeat, diff, editHeat, fromDetail, heatChanged, isLocked, merge, moveHeat, place, placedIn, pushHistory, redo, resetAll, undo, unlock, unplace, type ArrangeDraft, type DraftHeat, type Edit, type History } from "./draft";
import { AthleteChip, dragType, HeatCard } from "./HeatCard";
import { HeatDialog } from "./HeatDialog";
import { RoundStartDialog } from "./RoundStartDialog";
import { ScheduleDialog } from "./ScheduleDialog";

/** The arrange page: a local draft with undo and redo, saved in one go. */
export function ArrangeTab({ detail, meetId, onStale }: { detail: EventDetail; meetId: string; onStale: () => void }) {
  const event = detail.event;
  const laned = detail.rules.usesLanes;
  const [loaded, setLoaded] = useState(() => fromDetail(detail));
  const [history, setHistory] = useState<History>(() => ({ past: [], present: loaded, future: [] }));
  // Shared with the results page: switching tabs keeps the round.
  const [roundId, setRoundId] = useViewState(`event:round:${event.id}`, detail.rounds[0]?.id ?? "");
  const [picked, setPicked] = useState("");
  const [saving, setSaving] = useState(false);
  const [auto, setAuto] = useState(false);
  const [scheduling, setScheduling] = useState(false);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<DraftHeat | null>(null);
  const [starting, setStarting] = useState(false);
  const [unlocking, setUnlocking] = useState<DraftHeat | null>(null);
  const [resetting, setResetting] = useState(false);
  const [typed, setTyped] = useState("");
  const [rosterOver, setRosterOver] = useState(false);
  const draft = history.present;

  // Fresh data (a save, an entry, a write from elsewhere) is merged into the
  // draft heat by heat; local changes to untouched heats stay.
  const seen = useRef(detail);
  useEffect(() => {
    if (seen.current === detail) return;
    const previous = seen.current;
    seen.current = detail;
    if (saving) return;
    const fresh = fromDetail(detail);
    if (previous.rules.usesLanes !== detail.rules.usesLanes || (detail.rules.usesLanes && previous.event.laneCount !== detail.event.laneCount)) {
      if (diff(history.present, loaded, event.id).count > 0) toast("项目配置已修改", "warning");
      setHistory({ past: [], present: fresh, future: [] });
      setPicked("");
    } else {
      setHistory((h) => {
        const out = merge(h.present, loaded, fresh);
        if (out.dropped) toast("有组已在别处修改", "warning");
        return { past: [], present: out.draft, future: [] };
      });
    }
    setLoaded(fresh);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detail]);

  const { save, count } = useMemo(() => diff(draft, loaded, event.id), [draft, loaded, event.id]);
  const dirty = save.rounds.length > 0;
  useLeaveGuard(dirty);

  const apply = (next: ArrangeDraft) => setHistory((h) => pushHistory(h, next));
  const tryEdit = (edit: Edit) => {
    if (edit.error !== undefined) toast(edit.error, "warning");
    else apply(edit.draft);
    setPicked("");
  };
  const discard = () => setHistory({ past: [], present: loaded, future: [] });
  const submit = async () => {
    if (!dirty || saving) return;
    setSaving(true);
    try {
      const saved = await ArrangeController.save(save);
      const fresh = fromDetail(saved);
      setLoaded(fresh);
      setHistory({ past: [], present: fresh, future: [] });
      toast("已保存", "success");
    } catch (err) {
      // Changed elsewhere since it was loaded: the reload that follows merges.
      toast(errorText(err), isStale(err) ? "warning" : "danger");
      onStale();
    } finally {
      setSaving(false);
    }
  };

  const keys = useRef({ submit });
  keys.current = { submit };
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      const k = e.key.toLowerCase();
      if (document.querySelector("dialog[open]")) return;
      if (e.key === "Escape") setPicked("");
      else if (mod && k === "s") {
        e.preventDefault();
        void keys.current.submit();
      } else if (mod && ((k === "z" && e.shiftKey) || (k === "y" && e.ctrlKey))) {
        e.preventDefault();
        setHistory(redo);
      } else if (mod && k === "z") {
        e.preventDefault();
        setHistory(undo);
      }
    };
    window.addEventListener("keydown", down);
    return () => window.removeEventListener("keydown", down);
  }, []);

  const roundView = detail.rounds.find((r) => r.id === roundId) ?? detail.rounds[0];
  const round = draft.rounds.find((r) => r.roundId === roundView?.id);
  const loadedRound = loaded.rounds.find((r) => r.roundId === roundView?.id);
  const entries = useMemo(() => new Map<string, EntryView>(detail.entries.map((e) => [e.competitorId, e])), [detail.entries]);
  const placed = useMemo(() => placedIn(round), [round]);
  const conflicts = useDraftConflicts(event.id, meetId, draft.rounds);
  const rosterBox = useRef<HTMLDivElement>(null);
  const heatsBox = useRef<HTMLDivElement>(null);
  useScrollMemory(rosterBox, `arrange:roster:${event.id}:${roundView?.id}`);
  useScrollMemory(heatsBox, `arrange:heats:${event.id}:${roundView?.id}`);
  if (!roundView || !round) return <StEmpty>没有轮次</StEmpty>;
  const firstOpen = roundView.heats.find((h) => !h.keepReason);
  const free = detail.entries.filter((e) => !placed.has(e.competitorId));
  const stats = detail.rounds.reduce((n, r) => ({ heats: n.heats + r.heats.length, results: n.results + r.heats.reduce((m, h) => m + h.slots.filter((s) => s.result).length, 0) }), { heats: 0, results: 0 });

  const positionsOf = (h: DraftHeat) => {
    if (laned) return Array.from({ length: event.laneCount }, (_, i) => i + 1);
    return Array.from({ length: h.slots.length + (isLocked(draft, h) ? 0 : 1) }, (_, i) => i + 1);
  };

  return (
    <div className="fill view-enter" inert={saving}>
      <div className="subbar">
        {detail.rounds.length > 1 && <StSegmented aria-label="轮次" size="small" value={roundView.id} onChange={setRoundId} options={detail.rounds.map((r) => ({ value: r.id, label: r.label }))} />}
        <StButton variant="ghost" icon={<IconTime size={16} strokeWidth={iconStroke} />} title="轮次开始时间" disabled={saving} onClick={() => setStarting(true)}>
          {roundView.scheduledText || "开始时间"}
        </StButton>
        <span className="subbar__spacer" />
        <span className="subbar__count">{dirty ? `${count} 处改动` : ""}</span>
        <StIconButton label="撤销" disabled={history.past.length === 0} onClick={() => setHistory(undo)}>
          <IconUndo size={18} strokeWidth={iconStroke} />
        </StIconButton>
        <StIconButton label="重做" disabled={history.future.length === 0} onClick={() => setHistory(redo)}>
          <IconRedo size={18} strokeWidth={iconStroke} />
        </StIconButton>
        <StButton icon={<IconAuto size={16} strokeWidth={iconStroke} />} onClick={() => setAuto(true)}>自动编排</StButton>
        <StButton icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => apply(addHeat(draft, round.roundId))}>添加组</StButton>
        <StMenu
          trigger={(p) => (
            <StIconButton label="更多" {...p}>
              <IconMore size={18} strokeWidth={iconStroke} />
            </StIconButton>
          )}
          items={[
            { label: <span title={dirty ? "请先保存编排" : undefined}>批量排时间…</span>, disabled: dirty || saving || roundView.heats.length === 0, onSelect: () => setScheduling(true) },
            { label: "全部重置…", danger: true, disabled: stats.heats === 0 || draft.reset, onSelect: () => { setTyped(""); setResetting(true); } }]} />
        <StButton disabled={!dirty} onClick={discard}>放弃</StButton>
        {draft.reset ? (
          <StConfirmButton armedLook disabled={!dirty} loading={saving} onConfirm={submit}>重置并保存</StConfirmButton>
        ) : (
          <StButton variant="primary" disabled={!dirty} loading={saving} onClick={submit}>保存</StButton>
        )}
      </div>
      <StEnterScope>
        <div className="split">
          <aside className="split__side">
            <div className="split__side-head">
              <span>未分组</span>
              <span className="chip__meta">{free.length}/{detail.entries.length}</span>
            </div>
            <div ref={rosterBox} className="split__side-list drop-roster" data-over={rosterOver || undefined}
              onDragOver={(e) => { e.preventDefault(); setRosterOver(true); }} onDragLeave={() => setRosterOver(false)}
              onDrop={(e) => {
                e.preventDefault();
                setRosterOver(false);
                const id = e.dataTransfer.getData(dragType);
                if (id) tryEdit(unplace(draft, round.roundId, id, laned));
              }}
              onClick={() => picked && placed.has(picked) && tryEdit(unplace(draft, round.roundId, picked, laned))}>
              {free.map((e) => (
                <AthleteChip key={e.competitorId} entry={e} picked={picked === e.competitorId} meta={e.seedText || e.unitName} onPick={() => setPicked(picked === e.competitorId ? "" : e.competitorId)} />
              ))}
              {free.length === 0 && <StEmpty>{detail.entries.length === 0 ? "还没有报名" : picked ? "点这里移出" : "都已分组"}</StEmpty>}
            </div>
            <div className="split__side-foot">
              <StButton icon={<IconAdd size={16} strokeWidth={iconStroke} />} disabled={Boolean(event.combinedId)} onClick={() => setAdding(true)}>{detail.rules.team ? "管理队伍" : "从赛事添加"}</StButton>
            </div>
          </aside>
          <div ref={heatsBox} className="split__main" onClick={() => setPicked("")}>
            {round.heats.length === 0 && <StEmpty action={<StButton variant="primary" onClick={() => setAuto(true)}>自动编排</StButton>}>{draft.reset ? "保存后将删除全部组" : "还没有组"}</StEmpty>}
            <div className="heats">
              {round.heats.map((h, i) => (
                <HeatCard key={h.key} heat={h} index={i} count={round.heats.length} positions={positionsOf(h)} locked={isLocked(draft, h)} conflicts={conflicts.get(h.key)}
                  changed={heatChanged(h, loadedRound?.heats.find((x) => x.key === h.key))} entries={entries} picked={picked}
                  onPick={(id) => setPicked(picked === id ? "" : id)}
                  onPlace={(pos, id) => tryEdit(place(draft, round.roundId, h.key, pos, id, laned))}
                  onMove={(d) => apply(moveHeat(draft, round.roundId, h.key, d))}
                  onEdit={() => setEditing(h)} onDelete={() => tryEdit(deleteHeat(draft, round.roundId, h.key))} onUnlock={() => setUnlocking(h)} />
              ))}
            </div>
          </div>
        </div>
      </StEnterScope>
      <AutoDialog open={auto} detail={detail} round={roundView} exclude={[...placed.keys()]} onClose={() => setAuto(false)}
        onPlanned={(heats) => (heats.length === 0 ? toast("没有待编排报名", "warning") : apply(appendPlanned(draft, round.roundId, heats)))} />
      {scheduling && <ScheduleDialog day={detail.day} version={detail} onClose={() => setScheduling(false)}
        options={[{ eventId: event.id, roundId: roundView.id, label: `${event.name} ${roundView.label}`, firstAt: firstOpen?.effectiveAt ?? "", minutes: firstOpen?.durationMinutes ?? null }]} />}
      {starting && <RoundStartDialog eventId={event.id} round={roundView} day={detail.day} onClose={() => setStarting(false)} />}
      <AddEntriesDialog open={adding} detail={detail} meetId={meetId} onClose={() => setAdding(false)} />
      <HeatDialog heat={editing} round={round} eventId={event.id} day={detail.day} onClose={() => setEditing(null)} onSave={(patch) => editing && apply(editHeat(draft, round.roundId, editing.key, patch))} />
      <StDialog open={unlocking !== null} title="解锁位置？" size="small" onClose={() => setUnlocking(null)}
        actions={
          <>
            <StButton onClick={() => setUnlocking(null)}>取消</StButton>
            <StButton variant="danger" onClick={() => { if (unlocking) apply(unlock(draft, unlocking.key)); setUnlocking(null); }}>解锁</StButton>
          </>
        }>
        该组{unlocking?.keepReason}。移出报名者会删除对应成绩。
      </StDialog>
      <StDialog open={resetting} title="全部重置？" size="small" onClose={() => setResetting(false)}
        actions={
          <>
            <StButton onClick={() => setResetting(false)}>取消</StButton>
            <StButton variant="danger" disabled={typed.trim() !== event.name} onClick={() => { apply(resetAll(draft)); setResetting(false); }}>进入重置</StButton>
          </>
        }>
        <span>将删除 {stats.heats} 组与 {stats.results} 条成绩，保存后无法撤销。</span>
        <StTextField label={`输入「${event.name}」确认`} value={typed} onChange={setTyped} width="fill" />
      </StDialog>
    </div>
  );
}
