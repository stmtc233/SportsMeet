import { Fragment, useEffect } from "react";
import { ErrorBoundary } from "./app/ErrorBoundary";
import { NavProvider, meetTabLabels, sideTab, useNav, type MeetTab } from "./app/nav";
import { IconArrange, IconBack, IconEntries, IconEvent, IconMeet, IconOverview, IconPrint, IconResults, IconSettings, IconSetup, IconTime, StFieldFeedback, StListItem, StNavGroup, StNavHeading, StTag, StToastHost, iconStroke } from "./design";
import type { OverviewBadges } from "./mygo";
import { CombinedWorkspace } from "./features/combined/CombinedWorkspace";
import { EntriesPage } from "./features/entries/EntriesPage";
import { EventWorkspace } from "./features/event/EventWorkspace";
import { EventsPage } from "./features/events/EventsPage";
import { MeetsPage } from "./features/meets/MeetsPage";
import { useMeet } from "./features/meets/data";
import { OverviewPage } from "./features/overview/OverviewPage";
import { useOverview } from "./features/overview/data";
import { ReportsPage } from "./features/reports/ReportsPage";
import { ArrangePage } from "./features/rounds/ArrangePage";
import { ResultsPage } from "./features/rounds/ResultsPage";
import { SchedulePage } from "./features/schedule/SchedulePage";
import { SettingsPage } from "./features/settings/SettingsPage";
import { usePreferences } from "./features/settings/controller";
import { SetupPage } from "./features/setup/SetupPage";
import { TeamScoringPage } from "./features/team-scoring/TeamScoringPage";

// The sidebar follows the meet's work in order: what is set up and entered
// before the meet, what happens on the day, and what is printed.
const sections: { title?: string; tabs: { tab: MeetTab; icon: typeof IconEvent }[] }[] = [
  { tabs: [{ tab: "overview", icon: IconOverview }] },
  { title: "赛前", tabs: [{ tab: "setup", icon: IconSetup }, { tab: "events", icon: IconEvent }, { tab: "entries", icon: IconEntries }, { tab: "arrange", icon: IconArrange }, { tab: "schedule", icon: IconTime }] },
  { title: "比赛", tabs: [{ tab: "results", icon: IconResults }, { tab: "team-scoring", icon: IconMeet }] },
  { title: "输出", tabs: [{ tab: "reports", icon: IconPrint }] },
];

// The counts by a page's name in the sidebar: what is left to do there.
const badgeOf: Partial<Record<MeetTab, keyof OverviewBadges>> = { entries: "entries", arrange: "arrange", schedule: "schedule", results: "results" };

/** The meet's pages by phase, with what is left to do on some of them. */
function MeetNav({ meetId, current }: { meetId: string; current: MeetTab }) {
  const { go } = useNav();
  const badges = useOverview(meetId).data?.badges;
  const badge = (tab: MeetTab) => {
    const key = badgeOf[tab];
    const n = key && badges ? badges[key] : 0;
    // Not part of the page's name: the count is on 总览 and the page itself.
    return n ? <span aria-hidden><StTag tone={tab === "schedule" ? "warning" : "neutral"}>{n}</StTag></span> : undefined;
  };
  return (
    <StNavGroup>
      {sections.map(({ title, tabs }) => (
        <Fragment key={title ?? ""}>
          {title && <StNavHeading>{title}</StNavHeading>}
          {tabs.map(({ tab, icon: Icon }) => (
            <StListItem key={tab} title={meetTabLabels[tab]} leading={<Icon size={18} strokeWidth={iconStroke} />} selected={tab === current}
              trailing={badge(tab)} onClick={() => go({ page: "meet", meetId, tab })} />
          ))}
        </Fragment>
      ))}
    </StNavGroup>
  );
}

function Shell() {
  const { route, go } = useNav();
  if (route.page === "meets") return <MeetsPage />;
  if (route.page === "settings") return <SettingsPage />;
  const meetId = route.meetId;
  const current = sideTab(route);
  return (
    <div className="app">
      <nav className="app__side">
        <MeetHeader meetId={meetId} />
        <MeetNav meetId={meetId} current={current} />
        <div className="app__side-spacer" />
        <StListItem title="设置" leading={<IconSettings size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "settings", section: null, returnTo: route })} />
        <StListItem title="全部赛事" leading={<IconBack size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "meets" })} />
      </nav>
      <main className="app__main">
        {route.page === "combined" && <CombinedWorkspace key={route.combinedId} meetId={meetId} combinedId={route.combinedId} returnTo={route.returnTo} />}
        {route.page === "event" && <EventWorkspace key={route.eventId} meetId={meetId} eventId={route.eventId} tab={route.tab} returnTo={route.returnTo} />}
        {route.page === "meet" && route.tab === "overview" && <OverviewPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "setup" && <SetupPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "events" && <EventsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "entries" && <EntriesPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "arrange" && <ArrangePage meetId={meetId} />}
        {route.page === "meet" && route.tab === "schedule" && <SchedulePage meetId={meetId} />}
        {route.page === "meet" && route.tab === "results" && <ResultsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "team-scoring" && <TeamScoringPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "reports" && <ReportsPage meetId={meetId} />}
      </main>
    </div>
  );
}

function MeetHeader({ meetId }: { meetId: string }) {
  const meet = useMeet(meetId).data;
  return (
    <div className="app__meet">
      {/* The subtitle line is kept while loading and without dates: the tabs below never shift. */}
      <StListItem title={<strong>{meet?.name ?? ""}</strong>} subtitle={meet?.datesText || " "} />
    </div>
  );
}

// A page that fails to render leaves the rest of the app usable.
function Guarded() {
  const { go } = useNav();
  return (
    <ErrorBoundary onReset={() => go({ page: "meets" })}>
      <Shell />
    </ErrorBoundary>
  );
}

export function App() {
  const { data } = usePreferences();
  useEffect(() => { if (data) document.documentElement.dataset.theme = data.theme; }, [data]);
  return (
    <NavProvider>
      <StFieldFeedback>
        <Guarded />
        <StToastHost />
      </StFieldFeedback>
    </NavProvider>
  );
}
