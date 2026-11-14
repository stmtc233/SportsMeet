import { useEffect, useState } from "react";
import { IconAdd, StButton, StEmpty, StMarkField, StTable, StTag, iconStroke } from "../../design";
import type { EntryView, EventDetail, Event } from "../../mygo";
import { EventsController } from "../events/controller";
import { AddEntriesDialog } from "./AddEntriesDialog";

function SeedCell({ eventId, event, entry }: { eventId: string; event: Event; entry: EntryView }) {
  const [text, setText] = useState(entry.seedText);
  useEffect(() => setText(entry.seedText), [entry.seedText]);
  return (
    <span onClick={(e) => e.stopPropagation()}>
      <StMarkField quiet event={event} kind={event.kind} value={text} onChange={setText} aria-label="报名成绩"
        onCommit={(tidy) => {
          if (tidy !== entry.seedText) void EventsController.setSeed(eventId, entry.competitorId, tidy);
        }} />
    </span>
  );
}

/** The event's entries: add from the meet, set entry marks, withdraw. */
export function EntriesTab({ detail, meetId }: { detail: EventDetail; meetId: string }) {
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState(new Set<string>());
  const relay = detail.rules.team;
  const noun = relay ? "队" : "人";
  const eventId = detail.event.id;
  // Only athletes not placed in a heat can be withdrawn.
  const removable = detail.entries.filter((e) => selected.has(e.competitorId) && !e.placed);
  const remove = async () => {
    const ok = await EventsController.removeEntries(eventId, removable.map((e) => e.competitorId));
    if (ok) setSelected(new Set());
  };
  return (
    <div className="fill view-enter">
      <div className="subbar">
        <span>{detail.entries.length} {noun}报名</span>
        <span className="subbar__spacer" />
        <StButton disabled={removable.length === 0 || Boolean(detail.event.combinedId)} onClick={remove} reserveWidthFor={[`取消报名 999 ${noun}`]}>
          {removable.length > 0 ? `取消报名 ${removable.length} ${noun}` : "取消报名"}
        </StButton>
        <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} disabled={Boolean(detail.event.combinedId)} onClick={() => setAdding(true)}>{detail.rules.team ? "管理队伍" : "从赛事添加"}</StButton>
      </div>
      <div className="filters" />
      <div className="table-card">
        <StTable<EntryView> fill rows={detail.entries} scrollKey={`entries:${eventId}`} rowKey={(e) => e.competitorId} rowLabel={(e) => e.name} selected={selected} onSelect={setSelected}
          empty={<StEmpty action={<StButton variant="primary" disabled={Boolean(detail.event.combinedId)} onClick={() => setAdding(true)}>{detail.rules.team ? "管理队伍" : "从赛事添加"}</StButton>}>还没有报名</StEmpty>}
          columns={[
            ...(!relay ? [{ key: "bib", title: "号码", width: "88px", render: (e: EntryView) => e.bib }] : []),
            { key: "name", title: relay ? "队伍" : "姓名", width: "minmax(120px, 1.2fr)", render: (e) => <span className="cell-strong">{e.name} {e.groupMismatch && <StTag tone="warning">跨组报名</StTag>} {e.overEvents && <StTag tone="warning">超出限报项数</StTag>} {e.overUnit && <StTag tone="warning">单位超出限报</StTag>} {relay && e.genderMismatch && <StTag tone="warning">性别不符</StTag>}</span> },
            { key: "gender", title: relay ? (detail.event.kind === "relay" ? "棒次" : "成员") : "性别", width: relay ? "minmax(160px, 2fr)" : "96px", render: (e) => relay ? detail.teams.find((t) => t.id === e.teamId)?.memberNames.join(detail.event.kind === "relay" ? " → " : "、") : (e.genderMismatch ? <StTag tone="warning">{e.gender}</StTag> : e.gender) },
            { key: "unit", title: "参赛单位", width: "minmax(120px, 1.5fr)", render: (e) => e.unitName },
            { key: "seed", title: "报名成绩", width: "124px", align: "end", render: (e) => <SeedCell eventId={eventId} event={detail.event} entry={e} /> },
            { key: "placed", title: "编排", width: "88px", render: (e) => (e.placed ? <StTag tone="info">已排</StTag> : "") },
          ]} />
      </div>
      <AddEntriesDialog open={adding} detail={detail} meetId={meetId} onClose={() => setAdding(false)} />
    </div>
  );
}
