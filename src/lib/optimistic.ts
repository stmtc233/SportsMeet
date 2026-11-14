import { useEffect, useRef, useState } from "react";

/**
 * Settings whose controls take effect at once: a control shows the new
 * value straight away and the saves go out one after another, each with
 * every change so far. A value stays shown as chosen until the reloaded
 * data carries it, and goes back if its save fails. Controls are not
 * disabled while saving: the whole panel would blink on every click.
 */
export function useOptimistic<T extends object>(data: T | undefined, write: (next: T) => Promise<unknown>) {
  const [pending, setPending] = useState<Partial<T>>({});
  const latest = useRef({ data, pending, write });
  latest.current = { data, pending, write };
  const queue = useRef(Promise.resolve());
  useEffect(() => {
    if (!data) return;
    setPending((old) => {
      const left = Object.fromEntries(Object.entries(old).filter(([k, v]) => data[k as keyof T] !== v)) as Partial<T>;
      return Object.keys(left).length === Object.keys(old).length ? old : left;
    });
  }, [data]);
  const save = (patch: Partial<T>) => {
    latest.current.pending = { ...latest.current.pending, ...patch };
    setPending((old) => ({ ...old, ...patch }));
    queue.current = queue.current.then(async () => {
      const { data: saved, pending: changes, write } = latest.current;
      if (!saved) return;
      const ok = await write({ ...saved, ...changes });
      if (!ok) setPending((old) => Object.fromEntries(Object.entries(old).filter(([k]) => !(k in patch))) as Partial<T>);
    });
  };
  return { data: data && { ...data, ...pending }, save, waitForSave: () => queue.current };
}
