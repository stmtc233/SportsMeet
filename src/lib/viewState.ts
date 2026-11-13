import { useCallback, useLayoutEffect, useState, type RefObject } from "react";

// What the user was looking at, kept across page switches: scroll
// positions and in-page choices (a round, a tab, a filter). The memory
// lasts while the app runs; choices that are worth keeping across a
// restart (`lasting`) also go to localStorage. None of it is data: a
// remembered value that no longer fits (a deleted round) is ignored by the
// page, which falls back to its default.

const memory = new Map<string, unknown>();
const prefix = "sportsmeet:view:";

function readLasting(key: string): unknown {
  try {
    const raw = localStorage.getItem(prefix + key);
    return raw === null ? undefined : JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function writeLasting(key: string, value: unknown) {
  try {
    localStorage.setItem(prefix + key, JSON.stringify(value));
  } catch {
    // Storage blocked or full: the value lasts until the app quits.
  }
}

/** The remembered value, or undefined. */
export function recall<T>(key: string, lasting = false): T | undefined {
  if (memory.has(key)) return memory.get(key) as T;
  if (!lasting) return undefined;
  const value = readLasting(key);
  if (value !== undefined) memory.set(key, value);
  return value as T | undefined;
}

export function remember(key: string, value: unknown, lasting = false) {
  memory.set(key, value);
  if (lasting) writeLasting(key, value);
}

/**
 * useState that comes back with the value it had when the page was left.
 * The key names the page and what it holds ("results:round:<eventId>").
 * Pass `lasting` for choices kept across a restart; their values must be
 * JSON. A Set is stored as one, and comes back as one.
 */
export function useViewState<T>(key: string, initial: T | (() => T), lasting = false): [T, (next: T | ((old: T) => T)) => void] {
  const [state, setState] = useState<{ key: string; value: T }>(() => ({ key, value: restore(key, initial, lasting) }));
  // A new key (another event in the same component) reads its own value.
  const current = state.key === key ? state.value : restore(key, initial, lasting);
  if (state.key !== key) setState({ key, value: current });
  const set = useCallback((next: T | ((old: T) => T)) => {
    setState((old) => {
      const value = typeof next === "function" ? (next as (old: T) => T)(old.value) : next;
      remember(old.key, encode(value), lasting);
      return { key: old.key, value };
    });
  }, [lasting]);
  return [current, set];
}

function restore<T>(key: string, initial: T | (() => T), lasting: boolean): T {
  const fallback = typeof initial === "function" ? (initial as () => T)() : initial;
  const saved = recall<unknown>(key, lasting);
  if (saved === undefined) return fallback;
  if (fallback instanceof Set) return (Array.isArray(saved) ? new Set(saved) : fallback) as T;
  return typeof saved === typeof fallback ? (saved as T) : fallback;
}

function encode(value: unknown) {
  return value instanceof Set ? [...value] : value;
}

const scrolls = new Map<string, number>();

/**
 * Keeps an element's scroll position under a key and puts it back when the
 * element comes back. The content usually arrives after the element, so
 * the position is retried as the content grows, until reached, until the
 * user scrolls, clicks or types, or for at most 1.5 s.
 */
export function useScrollMemory(ref: RefObject<HTMLElement | null>, key: string | undefined) {
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || !key) return;
    const target = scrolls.get(key) ?? 0;
    let restoring = target > 0;
    const save = () => {
      if (!restoring) scrolls.set(key, el.scrollTop);
    };
    const attempt = () => {
      if (!restoring) return;
      el.scrollTop = target;
      if (Math.abs(el.scrollTop - target) < 1) stop();
    };
    const content = new MutationObserver(attempt);
    const size = new ResizeObserver(attempt);
    const user = () => stop();
    const userEvents = ["wheel", "pointerdown", "keydown", "touchstart"] as const;
    const timer = setTimeout(() => stop(), 1500);
    function stop() {
      if (!restoring) return;
      restoring = false;
      content.disconnect();
      size.disconnect();
      clearTimeout(timer);
      userEvents.forEach((type) => el!.removeEventListener(type, user));
      save();
    }
    el.addEventListener("scroll", save, { passive: true });
    if (restoring) {
      content.observe(el, { childList: true, subtree: true });
      size.observe(el);
      userEvents.forEach((type) => el.addEventListener(type, user, { passive: true }));
      attempt();
    }
    return () => {
      // Leaving mid-restore keeps the position still wanted.
      restoring = false;
      content.disconnect();
      size.disconnect();
      clearTimeout(timer);
      userEvents.forEach((type) => el.removeEventListener(type, user));
      el.removeEventListener("scroll", save);
    };
  }, [ref, key]);
}
