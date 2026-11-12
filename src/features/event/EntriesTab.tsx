import { useEffect, useState } from "react";
import { IconAdd, StButton, StEmpty, StMarkField, StTable, StTag, iconStroke } from "../../design";
import type { EntryView, EventDetail, EventKind } from "../../mygo";
import { EventsController } from "../events/controller";
import { AddEntriesDialog } from "./AddEntriesDialog";

function SeedCell({ eventId, kind, entry }: { eventId: string; kind: EventKind; entry: EntryView }) {
  const [text, setText] = useState(entry.seedText);
  useEffect(() => setText(entry.seedText), [entry.seedText]);
  return (
    <span onClick={(e) => e.stopPropagation()}>
      <StMarkField kind={kind} value={text} onChange={setText} aria-label="报名成绩"
        onCommit={(tidy) => {
          if (tidy !== entry.seedText) void EventsController.setSeed(eventId, entry.athleteId, tidy);
        }} />
    </span>
  );
}

/** The event's entries: add from the meet, set entry marks, withdraw. */
export function EntriesTab({ detail, meetId }: { detail: EventDetail; meetId: string }) {
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState(new Set<string>());
  const eventId = detail.event.id;
  // Only athletes not placed in a heat can be withdrawn.
  const removable = detail.entries.filter((e) => selected.has(e.athleteId) && !e.placed);
  const remove = async () => {
    const ok = await EventsController.removeEntries(eventId, removable.map((e) => e.athleteId));
    if (ok) setSelected(new Set());
  };
  return (
    <div className="fill">
      <div className="subbar">
        <span>{detail.entries.length} 人报名</span>
        <span className="subbar__spacer" />
        <StButton disabled={removable.length === 0} onClick={remove} reserveWidthFor={["取消报名 999 人"]}>
          {removable.length > 0 ? `取消报名 ${removable.length} 人` : "取消报名"}
        </StButton>
        <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setAdding(true)}>从赛事添加</StButton>
      </div>
      <div className="filters" />
      <div className="table-card">
        <StTable<EntryView> fill rows={detail.entries} rowKey={(e) => e.athleteId} selected={selected} onSelect={setSelected}
          empty={<StEmpty action={<StButton variant="primary" onClick={() => setAdding(true)}>从赛事添加</StButton>}>还没有人报名</StEmpty>}
          columns={[
            { key: "bib", title: "号码", width: "88px", render: (e) => e.bib },
            { key: "name", title: "姓名", width: "minmax(120px, 1.2fr)", render: (e) => <span className="cell-strong">{e.name}</span> },
            { key: "gender", title: "性别", width: "96px", render: (e) => (e.genderMismatch ? <StTag tone="warning">{e.gender}</StTag> : e.gender) },
            { key: "unit", title: "参赛单位", width: "minmax(120px, 1.5fr)", render: (e) => e.unitName },
            { key: "seed", title: "报名成绩", width: "124px", align: "end", render: (e) => <SeedCell eventId={eventId} kind={detail.event.kind} entry={e} /> },
            { key: "placed", title: "编排", width: "88px", render: (e) => (e.placed ? <StTag tone="info">已排</StTag> : "") },
          ]} />
      </div>
      <AddEntriesDialog open={adding} detail={detail} meetId={meetId} onClose={() => setAdding(false)} />
    </div>
  );
}
