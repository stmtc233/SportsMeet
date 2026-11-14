import { useNav } from "../../app/nav";
import { IconAI, StButton, StListItem, StTag, iconStroke } from "../../design";
import { useMCPStatus } from "./controller";

/** The enabled state stays visible wherever the connection page is opened. */
export function AIMCPEntry({ sidebar = false }: { sidebar?: boolean }) {
  const { route, go } = useNav();
  const { data } = useMCPStatus();
  const enabled = data?.enabled ?? false;
  const open = () => go({ page: "ai-mcp", returnTo: route });
  const icon = <IconAI size={18} strokeWidth={iconStroke} />;
  const state = enabled ? <StTag tone="info">已开启</StTag> : undefined;
  return sidebar
    ? <StListItem title="AI/MCP" leading={icon} trailing={state} highlighted={enabled} onClick={open} />
    : <StButton variant={enabled ? "tonal" : "secondary"} icon={icon} onClick={open}>AI/MCP{state && <> {state}</>}</StButton>;
}
