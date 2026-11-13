import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { SettingsSection } from "../features/settings/SettingsPage";
import { StButton, StDialog } from "../design";
import { recall, remember } from "../lib/viewState";

export type MeetTab = "events" | "schedule" | "athletes" | "units" | "reports" | "exchange";
export type EventTab = "entries" | "arrange" | "results";

export type Route =
  | { page: "meets" }
  | { page: "settings"; section: SettingsSection | null; returnTo?: Route }
  | { page: "meet"; meetId: string; tab: MeetTab }
  | { page: "combineds"; meetId: string }
  | { page: "combined"; meetId: string; combinedId: string }
  | { page: "event"; meetId: string; eventId: string; tab: EventTab };

interface Nav {
  route: Route;
  /** Goes to a route; asks first when the current page has unsaved changes. */
  go: (route: Route) => void;
  setDirty: (dirty: boolean) => void;
}

const NavContext = createContext<Nav | null>(null);

const meetTabs: MeetTab[] = ["events", "schedule", "athletes", "units", "reports", "exchange"];

/** The meet's page last shown while the app runs: opening the meet again goes back there. */
export function lastMeetTab(meetId: string): MeetTab {
  const tab = recall<MeetTab>(`meet:tab:${meetId}`);
  return tab && meetTabs.includes(tab) ? tab : "events";
}

function shown(route: Route) {
  if (route.page === "meet") remember(`meet:tab:${route.meetId}`, route.tab);
  return route;
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
  const dirty = useRef(false);
  const go = useCallback((next: Route) => {
    if (dirty.current) setPending(next);
    else setRoute(shown(next));
  }, []);
  const setDirty = useCallback((v: boolean) => {
    dirty.current = v;
  }, []);
  const nav = useMemo(() => ({ route, go, setDirty }), [route, go, setDirty]);
  const leave = () => {
    if (!pending) return;
    dirty.current = false;
    setRoute(shown(pending));
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
