import { useState, type DragEvent } from "react";
import { IconDelete, IconDown, IconEdit, IconLock, IconMore, IconUnlock, IconUp, StIconButton, StMenu, StSection, StTag, iconStroke } from "../../design";
import type { EntryView } from "../../mygo";
import type { DraftHeat } from "./draft";

export const dragType = "application/x-sportsmeet-athlete";

export function AthleteChip({ entry, picked, dim, meta, onPick, fixed }: { entry: EntryView | undefined; picked: boolean; dim?: boolean; meta?: string; onPick: () => void; fixed?: boolean }) {
  if (!entry) return null;
  return (
    <button type="button" className="chip" draggable={!fixed} data-picked={picked || undefined} data-dim={dim || undefined} data-static={fixed || undefined}
      onDragStart={(e) => {
        e.dataTransfer.setData(dragType, entry.competitorId);
        e.dataTransfer.effectAllowed = "move";
      }}
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
export function HeatCard({ heat, index, count, positions, locked, changed, entries, picked, onPick, onPlace, onMove, onEdit, onDelete, onUnlock }: HeatCardProps) {
  const [over, setOver] = useState(0);
  const bySlot = new Map(heat.slots.map((s) => [s.position, s.competitorId]));
  const drop = (e: DragEvent, position: number) => {
    e.preventDefault();
    setOver(0);
    const id = e.dataTransfer.getData(dragType);
    if (id) onPlace(position, id);
  };
  const time = heat.scheduledAt.split("T")[1] ?? "";
  return (
    <StSection padded={false}
      title={
        <>
          {heat.name || `第${index + 1}组`}
          {time && <StTag>{time}</StTag>}
          {heat.keepReason && <StTag tone={locked ? "warning" : "neutral"}>{locked ? heat.keepReason : "已解锁"}</StTag>}
          {changed && <StTag tone="info">已改</StTag>}
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
            { label: "编辑名称与时间…", icon: <IconEdit size={16} strokeWidth={iconStroke} />, onSelect: onEdit },
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
  );
}
