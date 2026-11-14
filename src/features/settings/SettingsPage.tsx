import { useEffect, useRef } from "react";
import { useNav } from "../../app/nav";
import { recall, remember, useScrollMemory } from "../../lib/viewState";
import { IconBack, StIconButton, StListItem, StNavGroup, StPage, StToolbar, iconStroke } from "../../design";
import { AboutContent } from "../about/AboutContent";
import { AuditPanel } from "./AuditPanel";
import { MCPPanel } from "./MCPPanel";
import { PreferencesPanel } from "./PreferencesPanel";

export type SettingsSection = "general" | "arrange" | "print" | "mcp" | "log" | "about";
const sections: { value: SettingsSection; label: string }[] = [
  { value: "general", label: "通用" }, { value: "arrange", label: "编排" }, { value: "print", label: "打印" },
  { value: "mcp", label: "MCP" }, { value: "log", label: "操作日志" }, { value: "about", label: "关于" },
];

export function SettingsPage() {
  const { route, go } = useNav();
  const content = useRef<HTMLElement>(null);
  const chosen = route.page === "settings" ? route.section : null;
  // No section chosen (opened from a toolbar): a wide window shows the one last open.
  const last = recall<SettingsSection>("settings:section");
  const section = chosen ?? (last && sections.some((s) => s.value === last) ? last : "general");
  useEffect(() => { if (chosen) remember("settings:section", chosen); }, [chosen]);
  useScrollMemory(content, `settings:${section}`);
  if (route.page !== "settings") return null;
  const title = sections.find((s) => s.value === section)!.label;
  const back = () => go(route.returnTo ?? { page: "meets" });
  return (
    <StPage flush toolbar={<StToolbar title="设置" leading={<StIconButton label="返回" onClick={back}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton>} />}>
      <div className="settings" data-detail={route.section !== null || undefined}>
        <nav className="settings__nav" aria-label="设置分类">
          <StNavGroup>
            {sections.map((s) => <StListItem key={s.value} title={s.label} selected={s.value === section} onClick={() => go({ ...route, section: s.value })} />)}
          </StNavGroup>
        </nav>
        <main ref={content} className="settings__content">
          <div className="settings__back"><StIconButton label="设置分类" onClick={() => go({ ...route, section: null })}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton></div>
          <div key={section} className="narrow view-enter">
            <h2 className="settings__title">{title}</h2>
            {section === "mcp" ? <MCPPanel /> : section === "log" ? <AuditPanel /> : section === "about" ? <AboutContent /> : <PreferencesPanel section={section} />}
          </div>
        </main>
      </div>
    </StPage>
  );
}
