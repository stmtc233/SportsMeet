import { StButton, StLinkButton, StListItem, StNotice, StSection, StSwitchItem } from "../../design";
import { SettingsController, useAppInfo, useOptimisticPreferences } from "../settings/controller";

// Open-source software the app ships with; keep in step with go.mod and
// package.json (docs/agents/design-system.md「图标」).
const licenses = [
  { name: "mygo", license: "MIT" },
  { name: "SQLite", license: "Public Domain" },
  { name: "MCP Go SDK", license: "Apache-2.0 / MIT" },
  { name: "jsonschema-go", license: "MIT" },
  { name: "uritemplate", license: "BSD-3-Clause" },
  { name: "segmentio/encoding、asm", license: "MIT" },
  { name: "React", license: "MIT" },
  { name: "Lucide", license: "ISC" },
  { name: "purego", license: "Apache-2.0" },
  { name: "excelize、efp、nfp", license: "BSD-3-Clause" },
  { name: "mscfb、msoleps", license: "Apache-2.0" },
  { name: "go-deepcopy", license: "MIT" },
  { name: "golang.org/x/net、text、oauth2、sync、sys、time、crypto", license: "BSD-3-Clause" },
];

export function AboutContent() {
 const { data, error } = useAppInfo();
 const { data: preferences, error: preferencesError, save, waitForSave } = useOptimisticPreferences();
 return <>
 {error && <StNotice tone="danger">{error}</StNotice>}
 <StSection title="SportsMeet" padded={false}>
  <StListItem title="版本" subtitle={data?.version ?? ""} />
  <StListItem title="GitHub 仓库" subtitle="stmtc233/SportsMeet"
   trailing={data && <StLinkButton href={data.repositoryURL} onClick={(e) => { e.preventDefault(); void SettingsController.openRepository(); }}>打开仓库</StLinkButton>} />
  <StListItem title="数据保存在本机" />
 </StSection>
 <StSection title="软件更新" padded={false}>
  {preferencesError && <StNotice tone="danger">{preferencesError}</StNotice>}
  <StListItem title={data && !data.canSelfUpdate ? "手动更新" : "更新检查"} subtitle={data?.updateMessage}
   trailing={<StButton disabled={!data} reserveWidthFor={["检查更新", "查看发布"]}
    onClick={() => void waitForSave().then(SettingsController.checkForUpdates)}>{data && !data.canSelfUpdate ? "查看发布" : "检查更新"}</StButton>} />
  <StSwitchItem title="接收测试版" subtitle="测试版可能不稳定" checked={preferences?.updatePrereleases ?? false} disabled={!preferences}
   onChange={(updatePrereleases) => save({ updatePrereleases })} />
 </StSection>
 <StSection title="开源许可" padded={false}>
  {licenses.map((l) => <StListItem key={l.name} title={l.name} trailing={l.license} />)}
 </StSection>
 </>;
}
