import { useEffect, useState } from "react";
import type { Tone } from "./StLayout";

interface Toast {
  id: number;
  text: string;
  tone: Tone;
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
  setTimeout(() => publish(current.filter((x) => x.id !== t.id)), tone === "danger" ? 5000 : 2600);
}

/** Mounted once, at the app's root. */
export function StToastHost() {
  const [list, setList] = useState(current);
  useEffect(() => {
    listeners.add(setList);
    return () => void listeners.delete(setList);
  }, []);
  return (
    <div className="st-toasts" aria-live="polite">
      {list.map((t) => (
        <div key={t.id} className={`st-toast st-tone-${t.tone}`}>
          {t.text}
        </div>
      ))}
    </div>
  );
}
