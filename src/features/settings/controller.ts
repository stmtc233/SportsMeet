import { useData } from "../../lib/useData";
import { act } from "../../lib/action";
import { App, Settings, type Preferences } from "../../mygo";

export const usePreferences = () => useData(() => Settings.preferences(), [], (c) => c.kind === "settings");
export const useMCPStatus = () => useData(() => Settings.mcpStatus(), [], (c) => c.kind === "settings");
export const useAppInfo = () => useData(() => App.info(), [], () => false);

export const SettingsController = {
  save: (preferences: Preferences) => act(() => Settings.savePreferences(preferences)),
  configure: (enabled: boolean, port: number, rotate = false) => act(() => Settings.configureMCP(enabled, port, rotate), "已保存"),
  reveal: () => act(() => Settings.revealToken()),
  copyToken: () => act(() => Settings.copyToken(), "已复制令牌"),
  copyConnection: () => act(() => Settings.copyConnection(), "已复制连接信息"),
};
