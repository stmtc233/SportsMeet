import { useNav, type EventTab } from "../../app/nav";
import { IconBack, StIconButton, StNotice, StPage, StSegmented, StSelect, StToolbar, iconStroke } from "../../design";
import { ArrangeTab } from "../arrange/ArrangeTab";
import { useEventDetail, useEvents } from "../events/controller";
import { ResultsTab } from "../results/ResultsTab";
import { EntriesTab } from "./EntriesTab";

/** One event: 报名 / 编排 / 成绩, with a switcher to the meet's other events. */
export function EventWorkspace({ meetId, eventId, tab }: { meetId: string; eventId: string; tab: EventTab }) {
  const { go } = useNav();
  const { data: detail, error, reload } = useEventDetail(eventId, meetId);
  const { data: events } = useEvents(meetId);
  return (
    <StPage flush
      toolbar={
        <StToolbar
          leading={
            <StIconButton label="返回项目列表" onClick={() => go({ page: "meet", meetId, tab: "events" })}>
              <IconBack size={18} strokeWidth={iconStroke} />
            </StIconButton>
          }
          title={
            <StSelect value={eventId} width="auto" onChange={(id) => go({ page: "event", meetId, eventId: id, tab })}
              options={(events ?? []).map((r) => ({ value: r.event.id, label: r.event.name }))} placeholder={detail?.event.name ?? ""} />
          }>
          <StSegmented value={tab} onChange={(t) => go({ page: "event", meetId, eventId, tab: t })}
            options={[{ value: "entries", label: "报名" }, { value: "arrange", label: "编排" }, { value: "results", label: "成绩" }]} />
        </StToolbar>
      }>
      {error && <StNotice tone="danger">{error}</StNotice>}
      {detail && tab === "entries" && <EntriesTab detail={detail} meetId={meetId} />}
      {detail && tab === "arrange" && <ArrangeTab detail={detail} meetId={meetId} onStale={reload} />}
      {detail && tab === "results" && <ResultsTab detail={detail} />}
    </StPage>
  );
}
