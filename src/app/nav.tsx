import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { StButton, StDialog } from "../design";

export type MeetTab = "events" | "athletes" | "units" | "reports" | "exchange";
export type EventTab = "entries" | "arrange" | "results";

export type Route =
  | { page: "meets" }
  | { page: "about" }
  | { page: "meet"; meetId: string; tab: MeetTab }
  | { page: "event"; meetId: string; eventId: string; tab: EventTab };

interface Nav {
  route: Route;
  /** Goes to a route; asks first when the current page has unsaved changes. */
  go: (route: Route) => void;
  setDirty: (dirty: boolean) => void;
}

const NavContext = createContext<Nav | null>(null);

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
    else setRoute(next);
  }, []);
  const setDirty = useCallback((v: boolean) => {
    dirty.current = v;
  }, []);
  const nav = useMemo(() => ({ route, go, setDirty }), [route, go, setDirty]);
  const leave = () => {
    if (!pending) return;
    dirty.current = false;
    setRoute(pending);
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
