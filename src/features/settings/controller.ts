import { useEffect, useRef, useState } from "react";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";
import { App, Settings, type Preferences } from "../../mygo";
import { appendOlder, mergeNewest, type AuditLog } from "./auditLog";

export const usePreferences = () => useData(() => Settings.preferences(), [], (c) => c.kind === "settings", "preferences");
export const useMCPStatus = () => useData(() => Settings.mcpStatus(), [], (c) => c.kind === "settings", "mcpStatus");
export const useAppInfo = () => useData(() => App.info(), [], () => false, "appInfo");

/**
 * Each control takes effect at once: it shows the new value straight away
 * and the saves go out one after another, each with every change so far.
 * A value stays shown as chosen until the reloaded settings carry it, and
 * goes back if its save fails. Controls are not disabled while saving: the
 * whole panel would blink on every click.
 */
export function useOptimisticPreferences() {
  const { data, error } = usePreferences();
  const [pending, setPending] = useState<Partial<Preferences>>({});
  const latest = useRef({ data, pending });
  latest.current = { data, pending };
  const queue = useRef(Promise.resolve());
  useEffect(() => {
    if (!data) return;
    setPending((old) => {
      const left = Object.fromEntries(Object.entries(old).filter(([k, v]) => data[k as keyof Preferences] !== v));
      return Object.keys(left).length === Object.keys(old).length ? old : left;
    });
  }, [data]);
  const save = (patch: Partial<Preferences>) => {
    latest.current.pending = { ...latest.current.pending, ...patch };
    setPending((old) => ({ ...old, ...patch }));
    queue.current = queue.current.then(async () => {
      const { data: saved, pending: changes } = latest.current;
      if (!saved) return;
      const ok = await SettingsController.save({ ...saved, ...changes });
      if (!ok) setPending((old) => Object.fromEntries(Object.entries(old).filter(([k]) => !(k in patch))));
    });
  };
  return { data: data && { ...data, ...pending }, error, save, waitForSave: () => queue.current };
}

const auditPage = 50;

/**
 * The operation log for a meet ("" = all) and a source ("" = all). Every
 * write logs a row, so any data change reads the newest page again; older
 * pages already loaded are kept.
 */
export function useAuditLog(meetId: string, source: string) {
  const key = `${meetId}|${source}`;
  // The page carries its filter: right after a filter change the hook still holds the previous one.
  const newest = useData(async () => ({ key, page: (await Settings.auditLog({ meetId, source, after: null, limit: auditPage }))! }), [key], () => true);
  const [log, setLog] = useState<{ key: string; log: AuditLog } | null>(null);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    const loaded = newest.data;
    if (!loaded || loaded.key !== key) return;
    setLog((current) => ({ key, log: mergeNewest(current?.key === key ? current.log : null, loaded.page) }));
  }, [newest.data, key]);
  const current = log?.key === key ? log.log : null;
  const more = async () => {
    if (!current?.next || loading) return;
    setLoading(true);
    const page = await act(async () => (await Settings.auditLog({ meetId, source, after: current.next, limit: auditPage }))!);
    setLoading(false);
    if (page) setLog((l) => (l?.key === key ? { key, log: appendOlder(l.log, page) } : l));
  };
  return { log: current, error: newest.error, loading, more };
}

export const SettingsController = {
  save: (preferences: Preferences) => act(() => Settings.savePreferences(preferences)),
  configure: (enabled: boolean, port: number, rotate = false) => act(() => Settings.configureMCP(enabled, port, rotate), "已保存"),
  reveal: () => act(() => Settings.revealToken()),
  copyToken: () => act(() => Settings.copyToken(), "已复制令牌"),
  checkForUpdates: () => act(() => App.checkForUpdates()),
  openRepository: () => act(() => App.openRepository()),
  copyConnection: () => act(() => Settings.copyConnection(), "已复制连接信息"),
};
