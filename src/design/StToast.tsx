import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { Tone } from "./StLayout";
import { easeOut, motionMs } from "./motion";

interface Toast {
  id: number;
  text: string;
  tone: Tone;
  leaving?: boolean;
}

let next = 1;
let current: Toast[] = [];
const listeners = new Set<(list: Toast[]) => void>();

function publish(list: Toast[]) {
  current = list;
  listeners.forEach((fn) => fn(list));
}

/** Shows the result of an action for a few seconds (15 characters at most). */
export function toast(text: string, tone: Tone = "neutral") {
  const t = { id: next++, text, tone };
  publish([...current.slice(-2), t]);
  // Fades out first (components.css), then leaves the list.
  setTimeout(() => {
    publish(current.map((x) => (x.id === t.id ? { ...x, leaving: true } : x)));
    setTimeout(() => publish(current.filter((x) => x.id !== t.id)), leaveMs);
  }, tone === "danger" ? 5000 : 2600);
}

/** --st-motion-fast. */
const leaveMs = 100;

/** Mounted once, at the app's root. */
export function StToastHost() {
  const [list, setList] = useState(current);
  useEffect(() => {
    listeners.add(setList);
    return () => void listeners.delete(setList);
  }, []);
  // Toasts already shown glide up when a new one pushes them, instead of jumping.
  const box = useRef<HTMLDivElement>(null);
  const tops = useRef(new Map<string, number>());
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const ms = motionMs("medium");
    const next = new Map<string, number>();
    for (const t of el.children as HTMLCollectionOf<HTMLElement>) {
      const id = t.dataset.id!;
      const top = t.offsetTop;
      next.set(id, top);
      const was = tops.current.get(id);
      if (ms > 0 && was !== undefined && was !== top) t.animate([{ transform: `translateY(${was - top}px)` }, { transform: "none" }], { duration: ms, easing: easeOut(), composite: "add" });
    }
    tops.current = next;
  }, [list]);
  return (
    <div ref={box} className="st-toasts" aria-live="polite">
      {list.map((t) => (
        <div key={t.id} data-id={t.id} className={`st-toast st-tone-${t.tone}`} data-leaving={t.leaving || undefined}>
          {t.text}
        </div>
      ))}
    </div>
  );
}
