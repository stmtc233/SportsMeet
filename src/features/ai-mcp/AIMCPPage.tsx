import { useNav } from "../../app/nav";
import { IconBack, StIconButton, StPage, StToolbar, iconStroke } from "../../design";
import { MCPPanel } from "./MCPPanel";

export function AIMCPPage() {
  const { route, go } = useNav();
  const back = () => go(route.page === "ai-mcp" && route.returnTo ? route.returnTo : { page: "meets" });
  return (
    <StPage scrollKey="ai-mcp" toolbar={<StToolbar title="AI/MCP" leading={<StIconButton label="返回" onClick={back}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton>} />}>
      <div className="narrow"><MCPPanel /></div>
    </StPage>
  );
}
