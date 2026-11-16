import { useNav } from "../../../app/nav";
import { remember } from "../../../lib/viewState";
import { styleJumpKey, selectedStyleKey } from "./keys";

/** Opens 设置 → 打印样式 on a style (「管理样式」 in 赛事设置); back returns to where it was opened from. */
export function useManageStyles() {
  const { route, go } = useNav();
  return (styleId: string) => {
    remember(selectedStyleKey, styleId);
    remember(styleJumpKey, true);
    go({ page: "settings", section: "print-styles", returnTo: route });
  };
}
