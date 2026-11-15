import { useNav } from "../../app/nav";
import { scrollOnOpen } from "../../lib/viewState";

/** The 「打印」 group of 赛事设置. */
export const printSettingsId = "setup-print";

/** The scroll key of 赛事设置 (SetupPage). */
export const setupScrollKey = (meetId: string) => `setup:${meetId}`;

/** Opens 赛事设置 at its 「打印」 group. */
export function useOpenPrintSettings(meetId: string) {
  const { go } = useNav();
  return () => {
    scrollOnOpen(setupScrollKey(meetId), printSettingsId);
    go({ page: "meet", meetId, tab: "setup" });
  };
}
