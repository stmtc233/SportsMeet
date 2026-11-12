import { useCallback, useEffect, useRef, useState } from "react";
import { events, type Change } from "../mygo";
import { coalesce } from "./coalesce";
import { errorText } from "./errors";

export interface Data<T> {
  data: T | undefined;
  error: string;
  reload: () => void;
}

// Loads data and reloads it when a matching dataChanged event arrives;
// reloads caused by events are merged (the first at once, then at most one
// per 500ms).
export function useData<T>(load: () => Promise<T>, deps: unknown[], match: (c: Change) => boolean): Data<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState("");
  const loadRef = useRef(load);
  const matchRef = useRef(match);
  loadRef.current = load;
  matchRef.current = match;
  const epoch = useRef(0);

  const run = useCallback(async () => {
    const mine = ++epoch.current;
    try {
      const out = await loadRef.current();
      if (mine === epoch.current) {
        setData(out);
        setError("");
      }
    } catch (err) {
      if (mine === epoch.current) setError(errorText(err));
    }
  }, []);

  useEffect(() => {
    setData(undefined);
    void run();
    const merged = coalesce(run);
    return events.dataChanged.on((c) => {
      if (matchRef.current(c)) merged();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { data, error, reload: run };
}
