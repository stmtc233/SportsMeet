import { createContext, useContext, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";

/** A motion token in milliseconds; 0 when the system asks for reduced motion. */
export function motionMs(token: "fast" | "medium" | "slow") {
  return parseFloat(getComputedStyle(document.documentElement).getPropertyValue(`--st-motion-${token}`)) || 0;
}

/** --st-ease-out, for animations started from script. */
export function easeOut() {
  return getComputedStyle(document.documentElement).getPropertyValue("--st-ease-out").trim() || "ease-out";
}

/**
 * The background under the selected child slides between children once
 * measured. It is placed without a transition first, and slides from then
 * on; while the container is not laid out (a hidden panel) there is no
 * thumb and the selected child paints its own background.
 */
export function useSlideThumb<T extends HTMLElement>(axis: "x" | "y", attr: "aria-checked" | "aria-current") {
  const ref = useRef<T>(null);
  const [box, setBox] = useState<{ at: number; size: number } | null>(null);
  const [animate, setAnimate] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const selector = attr === "aria-checked" ? ':scope > [aria-checked="true"]' : ":scope > [aria-current]";
    const measure = () => {
      const on = el.querySelector<HTMLElement>(selector);
      const next = on && on.offsetWidth > 0 ? (axis === "x" ? { at: on.offsetLeft, size: on.offsetWidth } : { at: on.offsetTop, size: on.offsetHeight }) : null;
      setBox((old) => (old?.at === next?.at && old?.size === next?.size ? old : next));
    };
    const resize = new ResizeObserver(measure);
    const watch = () => {
      resize.disconnect();
      resize.observe(el);
      for (const child of el.children) resize.observe(child);
    };
    const changes = new MutationObserver((records) => {
      if (records.some((r) => r.type === "childList")) watch();
      measure();
    });
    changes.observe(el, { childList: true, subtree: true, attributes: true, attributeFilter: [attr] });
    watch();
    measure();
    return () => {
      resize.disconnect();
      changes.disconnect();
    };
  }, [axis, attr]);
  const ready = box !== null;
  useEffect(() => {
    if (!ready) {
      setAnimate(false);
      return;
    }
    const frame = requestAnimationFrame(() => setAnimate(true));
    return () => cancelAnimationFrame(frame);
  }, [ready]);
  const style = box ? ({ "--thumb-at": `${box.at}px`, "--thumb-size": `${box.size}px` } as CSSProperties) : undefined;
  return { ref, style, animate };
}

const Settled = createContext<{ current: boolean } | null>(null);

/**
 * Inside a scope, what is already there when it first shows stays still;
 * only what mounts later (a dropped athlete, a new heat, an unfolded day)
 * plays its entrance (useEnter).
 */
export function StEnterScope({ children }: { children: ReactNode }) {
  const settled = useRef(false);
  useEffect(() => { settled.current = true; }, []);
  return <Settled.Provider value={settled}>{children}</Settled.Provider>;
}

/** True when this component mounted after its StEnterScope first showed; outside a scope, always. */
export function useEnter() {
  const settled = useContext(Settled);
  return useState(() => settled?.current ?? true)[0];
}

/**
 * Plays a short highlight when value changes (a recomputed rank, a new
 * qualification mark); nothing on first show.
 */
export function StChanged({ value, children }: { value: unknown; children: ReactNode }) {
  const last = useRef(value);
  const [times, setTimes] = useState(0);
  useEffect(() => {
    if (Object.is(last.current, value)) return;
    last.current = value;
    setTimes((n) => n + 1);
  }, [value]);
  return <span key={times} className="st-changed" data-changed={times > 0 || undefined}>{children}</span>;
}
