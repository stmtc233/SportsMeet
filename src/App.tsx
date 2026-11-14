import { useEffect, useState } from "react";
import { ErrorBoundary } from "./app/ErrorBoundary";
import { NavProvider, useNav, type MeetTab } from "./app/nav";
import { IconAthletes, IconBack, IconEdit, IconEntries, IconEvent, IconExport, IconGroup, IconMeet, IconOverview, IconPrint, IconSettings, IconTime, IconUnit, StFieldFeedback, StListItem, StNavGroup, StToastHost, iconStroke } from "./design";
import { AthletesPage } from "./features/athletes/AthletesPage";
import { CombinedList } from "./features/combined/CombinedList";
import { CombinedWorkspace } from "./features/combined/CombinedWorkspace";
import { EntriesPage } from "./features/entries/EntriesPage";
import { EventWorkspace } from "./features/event/EventWorkspace";
import { EventsPage } from "./features/events/EventsPage";
import { ExchangePage } from "./features/exchange/ExchangePage";
import { GroupsPage } from "./features/groups/GroupsPage";
import { MeetDialog } from "./features/meets/MeetDialog";
import { MeetsPage } from "./features/meets/MeetsPage";
import { useMeet } from "./features/meets/data";
import { OverviewPage } from "./features/overview/OverviewPage";
import { ReportsPage } from "./features/reports/ReportsPage";
import { SchedulePage } from "./features/schedule/SchedulePage";
import { SettingsPage } from "./features/settings/SettingsPage";
import { usePreferences } from "./features/settings/controller";
import { TeamScoringPage } from "./features/team-scoring/TeamScoringPage";
import { UnitsPage } from "./features/units/UnitsPage";

const tabs: { tab: MeetTab; label: string; icon: typeof IconEvent }[] = [
  { tab: "overview", label: "总览", icon: IconOverview },
  { tab: "events", label: "项目", icon: IconEvent },
  { tab: "entries", label: "报名", icon: IconEntries },
  { tab: "schedule", label: "日程", icon: IconTime },
  { tab: "athletes", label: "运动员", icon: IconAthletes },
  { tab: "units", label: "参赛单位", icon: IconUnit },
  { tab: "groups", label: "组别", icon: IconGroup },
  { tab: "team-scoring", label: "团体总分", icon: IconMeet },
  { tab: "reports", label: "打印", icon: IconPrint },
  { tab: "exchange", label: "导入导出", icon: IconExport },
];

function Shell() {
  const { route, go } = useNav();
  if (route.page === "meets") return <MeetsPage />;
  if (route.page === "settings") return <SettingsPage />;
  const meetId = route.meetId;
  const current: MeetTab = route.page === "meet" ? route.tab : "events";
  return (
    <div className="app">
      <nav className="app__side">
        <MeetHeader meetId={meetId} />
        <StNavGroup>
          {tabs.map(({ tab, label, icon: Icon }) => (
            <StListItem key={tab} title={label} leading={<Icon size={18} strokeWidth={iconStroke} />} selected={tab === current} onClick={() => go({ page: "meet", meetId, tab })} />
          ))}
        </StNavGroup>
        <div className="app__side-spacer" />
        <EditMeet meetId={meetId} />
        <StListItem title="设置" leading={<IconSettings size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "settings", section: null, returnTo: route })} />
        <StListItem title="全部赛事" leading={<IconBack size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "meets" })} />
      </nav>
      <main className="app__main">
        {route.page === "combineds" && <CombinedList meetId={meetId} />}
        {route.page === "combined" && <CombinedWorkspace key={route.combinedId} meetId={meetId} combinedId={route.combinedId} />}
        {route.page === "event" && <EventWorkspace key={route.eventId} meetId={meetId} eventId={route.eventId} tab={route.tab} />}
        {route.page === "meet" && route.tab === "groups" && <GroupsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "events" && <EventsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "overview" && <OverviewPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "entries" && <EntriesPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "schedule" && <SchedulePage meetId={meetId} />}
        {route.page === "meet" && route.tab === "athletes" && <AthletesPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "units" && <UnitsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "team-scoring" && <TeamScoringPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "reports" && <ReportsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "exchange" && <ExchangePage meetId={meetId} />}
      </main>
    </div>
  );
}

function MeetHeader({ meetId }: { meetId: string }) {
  const meet = useMeet(meetId).data;
  return (
    <div className="app__meet">
      {/* The subtitle line is kept while loading and without dates: the tabs below never shift. */}
      <StListItem title={<strong>{meet?.name ?? ""}</strong>} subtitle={meet?.datesText || "\u00a0"} />
    </div>
  );
}

function EditMeet({ meetId }: { meetId: string }) {
  const meet = useMeet(meetId).data;
  const [open, setOpen] = useState(false);
  return (
    <>
      <StListItem title="编辑赛事" leading={<IconEdit size={18} strokeWidth={iconStroke} />} disabled={!meet} onClick={() => setOpen(true)} />
      <MeetDialog open={open && !!meet} meet={meet ?? undefined} onClose={() => setOpen(false)} />
    </>
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

