import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { SettingsSection } from "../features/settings/SettingsPage";
import { StButton, StDialog } from "../design";
import { recall, remember } from "../lib/viewState";

export type MeetTab = "overview" | "setup" | "events" | "entries" | "arrange" | "schedule" | "results" | "team-scoring" | "reports";
export type EventTab = "entries" | "arrange" | "results";

export type Route =
  | { page: "meets" }
  | { page: "settings"; section: SettingsSection | null; returnTo?: Route }
  | { page: "meet"; meetId: string; tab: MeetTab }
  | { page: "combined"; meetId: string; combinedId: string; returnTo?: Route }
  | { page: "event"; meetId: string; eventId: string; tab: EventTab; returnTo?: Route };

interface Nav {
  route: Route;
  /** Goes to a route; asks first when the current page has unsaved changes. */
  go: (route: Route) => void;
  setDirty: (dirty: boolean) => void;
}

const NavContext = createContext<Nav | null>(null);

/** The meet's pages by their sidebar names. */
export const meetTabLabels: Record<MeetTab, string> = {
  overview: "总览", setup: "赛事设置", events: "项目", entries: "报名", arrange: "编排", schedule: "日程", results: "成绩", "team-scoring": "团体总分", reports: "打印",
};

const meetTabs = Object.keys(meetTabLabels) as MeetTab[];

/** Where a back button goes, and its name: the page it was opened from, else the fallback. */
export function backTo(returnTo: Route | undefined, fallback: Route): { route: Route; label: string } {
  const route = returnTo ?? fallback;
  return { route, label: route.page === "meet" ? `返回${meetTabLabels[route.tab]}` : route.page === "combined" ? "返回全能" : "返回" };
}

/** The meet's page last shown while the app runs: opening the meet again goes back there. */
export function lastMeetTab(meetId: string): MeetTab {
  const tab = recall<MeetTab>(`meet:tab:${meetId}`);
  return tab && meetTabs.includes(tab) ? tab : "overview";
}

/** The sidebar page a route belongs to: an event or combined event opened from a page stays under it. */
export function sideTab(route: Route): MeetTab {
  if (route.page === "meet") return route.tab;
  if ((route.page === "event" || route.page === "combined") && route.returnTo) return sideTab(route.returnTo);
  return "events";
}

/**
 * An event or combined event remembers where it was opened from, so its
 * back button goes there: a meet page, or the combined event of a
 * component. Switching tabs or events keeps the place it came from.
 */
function withReturn(current: Route, next: Route): Route {
  if ((next.page !== "event" && next.page !== "combined") || next.returnTo) return next;
  if (current.page === "meet") return { ...next, returnTo: current };
  if (current.page === "event" && next.page === "event") return { ...next, returnTo: current.returnTo };
  if (current.page === "combined" && next.page === "combined") return { ...next, returnTo: current.returnTo };
  if (current.page === "combined" && next.page === "event") return { ...next, returnTo: current };
  return next;
}

function shown(route: Route) {
  if (route.page === "meet") remember(`meet:tab:${route.meetId}`, route.tab);
  if (route.page === "event") remember(`event:tab:${route.eventId}`, route.tab);
  return route;
}

/** The tab an event opens at from a list: the one last used for it, else 报名. */
export function lastEventTab(eventId: string): EventTab {
  const tab = recall<EventTab>(`event:tab:${eventId}`);
  return tab === "arrange" || tab === "results" ? tab : "entries";
}

export function useNav(): Nav {
  const nav = useContext(NavContext);
  if (!nav) throw new Error("useNav outside NavProvider");
  return nav;
}

/** Marks the page as having unsaved changes: leaving it asks first. */
export function useLeaveGuard(dirty: boolean) {
  const { setDirty } = useNav();
  useEffect(() => {
    setDirty(dirty);
    return () => setDirty(false);
  }, [dirty, setDirty]);
}

export function NavProvider({ children }: { children: ReactNode }) {
  const [route, setRoute] = useState<Route>({ page: "meets" });
  const [pending, setPending] = useState<Route | null>(null);
  const current = useRef(route);
  const dirty = useRef(false);
  const show = useCallback((next: Route) => {
    current.current = next;
    setRoute(shown(next));
  }, []);
  const go = useCallback((target: Route) => {
    const next = withReturn(current.current, target);
    if (dirty.current) setPending(next);
    else show(next);
  }, [show]);
  const setDirty = useCallback((v: boolean) => {
    dirty.current = v;
  }, []);
  const nav = useMemo(() => ({ route, go, setDirty }), [route, go, setDirty]);
  const leave = () => {
    if (!pending) return;
    dirty.current = false;
    show(pending);
    setPending(null);
  };
  return (
    <NavContext.Provider value={nav}>
      {children}
      <StDialog open={pending !== null} title="放弃未保存的改动？" size="small" onClose={() => setPending(null)}
        actions={
          <>
            <StButton onClick={() => setPending(null)}>继续编辑</StButton>
            <StButton variant="danger" onClick={leave}>放弃</StButton>
          </>
        }>
        离开后改动不会保存。
      </StDialog>
    </NavContext.Provider>
  );
}
