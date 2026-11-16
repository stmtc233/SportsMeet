import { useEffect, useRef } from "react";
import { useNav } from "../../app/nav";
import { recall, remember, useScrollMemory } from "../../lib/viewState";
import { IconBack, StIconButton, StListItem, StNavGroup, StPage, StToolbar, iconStroke } from "../../design";
import { AboutContent } from "../about/AboutContent";
import { AuditPanel } from "./AuditPanel";
import { PreferencesPanel } from "./PreferencesPanel";
import { PrintStylesPanel } from "./print-styles/PrintStylesPanel";

export type SettingsSection = "general" | "print-styles" | "log" | "about";
const sections: { value: SettingsSection; label: string }[] = [
  { value: "general", label: "通用" }, { value: "print-styles", label: "打印样式" }, { value: "log", label: "操作日志" }, { value: "about", label: "关于" },
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
          <div key={section} className={section === "print-styles" ? "wide view-enter" : "narrow view-enter"}>
            <h2 className="settings__title">{title}</h2>
            {section === "log" ? <AuditPanel /> : section === "about" ? <AboutContent /> : section === "print-styles" ? <PrintStylesPanel /> : <PreferencesPanel />}
          </div>
        </main>
      </div>
    </StPage>
  );
}
