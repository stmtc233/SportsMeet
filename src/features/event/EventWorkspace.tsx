import { backTo, useNav, type EventTab, type Route } from "../../app/nav";
import { IconBack, IconChevronRight, StButton, StIconButton, StNotice, StPage, StSegmented, StSelect, StToolbar, iconStroke, toast } from "../../design";
import { recall } from "../../lib/viewState";
import { useOpenRound } from "../rounds/ArrangePage";
import { useMeetRounds } from "../rounds/controller";
import { ArrangeTab } from "../arrange/ArrangeTab";
import { useEventDetail, useEvents } from "../events/controller";
import { ResultsTab } from "../results/ResultsTab";
import { EntriesTab } from "./EntriesTab";

/** One event: 报名 / 编排 / 成绩, with a switcher to the meet's other events. */
export function EventWorkspace({ meetId, eventId, tab, returnTo }: { meetId: string; eventId: string; tab: EventTab; returnTo?: Route }) {
  const { go } = useNav();
  const { data: detail, error, reload } = useEventDetail(eventId, meetId);
  const { data: events } = useEvents(meetId);
  const combinedId = detail?.event.combinedId;
  const back = backTo(returnTo, combinedId ? { page: "combined", meetId, combinedId } : { page: "meet", meetId, tab: "events" });
  return (
    <StPage flush
      toolbar={
        <StToolbar
          leading={
            <StIconButton label={back.label} onClick={() => go(back.route)}>
              <IconBack size={18} strokeWidth={iconStroke} />
            </StIconButton>
          }
          title={
            <StSelect value={eventId} width="auto" onChange={(id) => go({ page: "event", meetId, eventId: id, tab })}
              options={(events ?? []).filter((r) => (r.event.combinedId ?? "") === (combinedId ?? "")).map((r) => ({ value: r.event.id, label: r.event.name }))} placeholder={detail?.event.name ?? ""} />
          }>
          {returnTo?.page === "meet" && returnTo.tab === "results" && <NextRound meetId={meetId} eventId={eventId} />}
          <StSegmented aria-label="项目页面" value={tab} onChange={(t) => go({ page: "event", meetId, eventId, tab: t })}
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

/**
 * 「下一场」 from 成绩: the round after the one shown, in the order of the
 * match day (by time, then those without one).
 */
function NextRound({ meetId, eventId }: { meetId: string; eventId: string }) {
  const { data } = useMeetRounds(meetId);
  const open = useOpenRound(meetId);
  const order = [...(data?.days ?? []).flatMap((d) => d.rounds), ...(data?.unscheduled ?? [])];
  const next = () => {
    const shown = recall<string>(`event:round:${eventId}`);
    let i = order.findIndex((r) => r.roundId === shown && r.eventId === eventId);
    if (i < 0) i = order.findIndex((r) => r.eventId === eventId);
    const n = order[i + 1];
    if (!n) return toast("已是最后一场");
    open(n, n.heats > 0 ? "results" : "arrange");
  };
  return (
    <StButton icon={<IconChevronRight size={16} strokeWidth={iconStroke} />} disabled={order.length < 2} onClick={next}>下一场</StButton>
  );
}
