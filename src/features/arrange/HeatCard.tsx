import { useState, type DragEvent } from "react";
import { IconDelete, IconDown, IconEdit, IconLock, IconMore, IconUnlock, IconUp, StIconButton, StMenu, StMuted, StSection, StTag, iconStroke, useEnter } from "../../design";
import { HeatTime } from "../event/HeatTime";
import type { EntryView, HeatConflict } from "../../mygo";
import type { DraftHeat } from "./draft";

export const dragType = "application/x-sportsmeet-athlete";

export function AthleteChip({ entry, picked, dim, meta, onPick, fixed }: { entry: EntryView | undefined; picked: boolean; dim?: boolean; meta?: string; onPick: () => void; fixed?: boolean }) {
  const enter = useEnter();
  const [dragging, setDragging] = useState(false);
  if (!entry) return null;
  return (
    <button type="button" className="chip" draggable={!fixed} data-picked={picked || undefined} data-dim={dim || undefined} data-static={fixed || undefined}
      data-enter={enter || undefined} data-dragging={dragging || undefined}
      onDragStart={(e) => {
        e.dataTransfer.setData(dragType, entry.competitorId);
        e.dataTransfer.effectAllowed = "move";
        setDragging(true);
      }}
      onDragEnd={() => setDragging(false)}
      onClick={(e) => {
        e.stopPropagation();
        onPick();
      }}>
      <span className="chip__name">{entry.name}</span>
      <span className="chip__meta">{meta ?? entry.unitName}</span>
    </button>
  );
}

interface HeatCardProps {
  heat: DraftHeat;
  index: number;
  count: number;
  positions: number[];
  locked: boolean;
  changed: boolean;
  /** From the draft, unsaved changes included: a warning, never a block. */
  conflicts?: HeatConflict[];
  entries: Map<string, EntryView>;
  picked: string;
  onPick: (competitorId: string) => void;
  onPlace: (position: number, competitorId: string) => void;
  onMove: (delta: -1 | 1) => void;
  onEdit: () => void;
  onDelete: () => void;
  onUnlock: () => void;
}

/** One heat: a row per position, each a drop and click target. */
export function HeatCard({ heat, index, count, positions, locked, changed, conflicts, entries, picked, onPick, onPlace, onMove, onEdit, onDelete, onUnlock }: HeatCardProps) {
  const [over, setOver] = useState(0);
  const enter = useEnter();
  const bySlot = new Map(heat.slots.map((s) => [s.position, s.competitorId]));
  const drop = (e: DragEvent, position: number) => {
    e.preventDefault();
    setOver(0);
    const id = e.dataTransfer.getData(dragType);
    if (id) onPlace(position, id);
  };
  return (
    <div className="heat" data-enter={enter || undefined}>
      <StSection padded={false}
        title={
          <>
            {heat.name || `第${index + 1}组`}
            <HeatTime text={heat.scheduledText} inherited={heat.scheduleInherited} />
            {heat.durationText && <StMuted>{heat.durationText}</StMuted>}
            {heat.keepReason && <StTag tone={locked ? "warning" : "neutral"}>{locked ? heat.keepReason : "已解锁"}</StTag>}
            {changed && <StTag tone="info">已改</StTag>}
            {conflicts && conflicts.length > 0 && <ConflictTag conflicts={conflicts} />}
          </>
        }
        actions={
          <StMenu
            trigger={(p) => (
              <StIconButton size="small" label="更多" {...p}>
                <IconMore size={16} strokeWidth={iconStroke} />
              </StIconButton>
            )}
            items={[
              { label: "编辑组…", icon: <IconEdit size={16} strokeWidth={iconStroke} />, onSelect: onEdit },
              { label: "上移", icon: <IconUp size={16} strokeWidth={iconStroke} />, onSelect: () => onMove(-1), disabled: index === 0 },
              { label: "下移", icon: <IconDown size={16} strokeWidth={iconStroke} />, onSelect: () => onMove(1), disabled: index === count - 1 },
              { label: "解锁位置…", icon: locked ? <IconUnlock size={16} strokeWidth={iconStroke} /> : <IconLock size={16} strokeWidth={iconStroke} />, onSelect: onUnlock, disabled: !locked },
              { label: "删除", icon: <IconDelete size={16} strokeWidth={iconStroke} />, onSelect: onDelete, danger: true, separated: true, disabled: heat.keepReason !== "" },
            ]} />
        }>
        <div className="heat-slots">
          {positions.map((pos) => {
            const competitorId = bySlot.get(pos);
            const target = picked !== "" && picked !== competitorId && !locked;
            return (
              <div key={pos} className="slot" data-over={over === pos || undefined} data-target={target || undefined}
                onDragOver={(e) => {
                  if (locked) return;
                  e.preventDefault();
                  setOver(pos);
                }}
                onDragLeave={() => setOver(0)} onDrop={(e) => drop(e, pos)}
                onClick={target ? () => onPlace(pos, picked) : undefined}>
                <span className="slot__pos">{pos}</span>
                <span className="slot__body">
                  {competitorId ? (
                    <AthleteChip entry={entries.get(competitorId)} picked={picked === competitorId} fixed={locked} onPick={() => (target ? onPlace(pos, picked) : onPick(competitorId))} />
                  ) : (
                    <span className="slot__empty">{target ? "放到这里" : ""}</span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      </StSection>
    </div>
  );
}

/** Marks a heat with athlete time conflicts; the title lists who and with which heat. */
export function ConflictTag({ conflicts }: { conflicts: HeatConflict[] }) {
  const lines = conflicts.map((c) => `${c.athleteName}：${c.text}，${c.other.text}`);
  return <span className="conflict-tag" title={lines.join("\n")} data-conflict><StTag tone="danger">冲突 {conflicts.length}</StTag></span>;
}
