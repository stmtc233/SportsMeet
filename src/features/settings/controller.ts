import { useEffect, useState } from "react";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";
import { App, Settings, type Preferences } from "../../mygo";
import { appendOlder, mergeNewest, type AuditLog } from "./auditLog";

export const usePreferences = () => useData(() => Settings.preferences(), [], (c) => c.kind === "settings");
export const useMCPStatus = () => useData(() => Settings.mcpStatus(), [], (c) => c.kind === "settings");
export const useAppInfo = () => useData(() => App.info(), [], () => false);

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
  copyConnection: () => act(() => Settings.copyConnection(), "已复制连接信息"),
};
