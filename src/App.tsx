import { useEffect } from "react";
import { usePreferences } from "./features/settings/controller";
import { NavProvider, useNav, type MeetTab } from "./app/nav";
import { IconAthletes, IconBack, IconEvent, IconExport, IconPrint, IconSettings, IconUnit, StListItem, StToastHost, iconStroke } from "./design";
import { SettingsPage } from "./features/settings/SettingsPage";
import { AthletesPage } from "./features/athletes/AthletesPage";
import { EventWorkspace } from "./features/event/EventWorkspace";
import { EventsPage } from "./features/events/EventsPage";
import { ExchangePage } from "./features/exchange/ExchangePage";
import { MeetsPage } from "./features/meets/MeetsPage";
import { useMeet } from "./features/meets/data";
import { ReportsPage } from "./features/reports/ReportsPage";
import { UnitsPage } from "./features/units/UnitsPage";
import { dateRange } from "./lib/labels";

const tabs: { tab: MeetTab; label: string; icon: typeof IconEvent }[] = [
  { tab: "events", label: "项目", icon: IconEvent },
  { tab: "athletes", label: "运动员", icon: IconAthletes },
  { tab: "units", label: "参赛单位", icon: IconUnit },
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
        {tabs.map(({ tab, label, icon: Icon }) => (
          <StListItem key={tab} title={label} leading={<Icon size={18} strokeWidth={iconStroke} />} selected={tab === current} onClick={() => go({ page: "meet", meetId, tab })} />
        ))}
        <div className="app__side-spacer" />
        <StListItem title="设置" leading={<IconSettings size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "settings", section: null, returnTo: route })} />
        <StListItem title="全部赛事" leading={<IconBack size={18} strokeWidth={iconStroke} />} onClick={() => go({ page: "meets" })} />
      </nav>
      <main className="app__main">
        {route.page === "event" && <EventWorkspace key={route.eventId} meetId={meetId} eventId={route.eventId} tab={route.tab} />}
        {route.page === "meet" && route.tab === "events" && <EventsPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "athletes" && <AthletesPage meetId={meetId} />}
        {route.page === "meet" && route.tab === "units" && <UnitsPage meetId={meetId} />}
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
      <StListItem title={<strong>{meet?.name ?? ""}</strong>} subtitle={meet ? dateRange(meet.startDate, meet.endDate) : ""} />
    </div>
  );
}

export function App() {
  const { data } = usePreferences();
  useEffect(() => { if (data) document.documentElement.dataset.theme = data.theme; }, [data]);
  return (
    <NavProvider>
      <Shell />
      <StToastHost />
    </NavProvider>
  );
}

