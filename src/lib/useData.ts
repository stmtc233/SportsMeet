import { useCallback, useEffect, useRef, useState } from "react";
import { events, type Change } from "../mygo";
import { coalesce } from "./coalesce";
import { errorText } from "./errors";

export interface Data<T> {
  data: T | undefined;
  error: string;
  reload: () => void;
}

// The last result of each keyed read. A page that comes back shows it at
// once and reads again at the same time, instead of opening empty and
// filling in a moment later. Only the newest few are kept.
const cache = new Map<string, unknown>();
const cacheSize = 40;

function cached<T>(key: string | undefined): T | undefined {
  return key === undefined ? undefined : (cache.get(key) as T | undefined);
}

function store(key: string, value: unknown) {
  cache.delete(key);
  cache.set(key, value);
  if (cache.size > cacheSize) cache.delete(cache.keys().next().value!);
}

// Loads data and reloads it when a matching dataChanged event arrives;
// reloads caused by events are merged (the first at once, then at most one
// per 500ms). With a cache key, a page shows the previous result of the
// same read until the new one arrives; the key must name everything the
// read depends on.
export function useData<T>(load: () => Promise<T>, deps: unknown[], match: (c: Change) => boolean, cacheKey?: string): Data<T> {
  const [data, setData] = useState<T | undefined>(() => cached<T>(cacheKey));
  const [error, setError] = useState("");
  const loadRef = useRef(load);
  const matchRef = useRef(match);
  const keyRef = useRef(cacheKey);
  loadRef.current = load;
  matchRef.current = match;
  keyRef.current = cacheKey;
  const epoch = useRef(0);

  const run = useCallback(async () => {
    const mine = ++epoch.current;
    const key = keyRef.current;
    try {
      const out = await loadRef.current();
      if (key !== undefined) store(key, out);
      if (mine === epoch.current) {
        setData(out);
        setError("");
      }
    } catch (err) {
      if (mine === epoch.current) setError(errorText(err));
    }
  }, []);

  const first = useRef(true);
  useEffect(() => {
    // The first render already holds the cached value.
    if (!first.current) setData(cached<T>(keyRef.current));
    first.current = false;
    void run();
    const merged = coalesce(run);
    return events.dataChanged.on((c) => {
      if (matchRef.current(c)) merged();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { data, error, reload: run };
}
