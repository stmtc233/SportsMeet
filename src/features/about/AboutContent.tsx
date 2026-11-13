import { StButton, StListItem, StNotice, StSection } from "../../design";
import { SettingsController, useAppInfo } from "../settings/controller";

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
  { name: "golang.org/x/net、text、oauth2、sync、sys、time", license: "BSD-3-Clause" },
];

export function AboutContent() {
 const { data, error } = useAppInfo();
 return <>
 {error && <StNotice tone="danger">{error}</StNotice>}
 <StSection title="SportsMeet" padded={false}>
  <StListItem title="版本" subtitle={data?.version ?? ""}
   trailing={<StButton onClick={SettingsController.checkForUpdates}>检查更新</StButton>} />
  <StListItem title="数据保存在本机" />
 </StSection>
 <StSection title="开源许可" padded={false}>
  {licenses.map((l) => <StListItem key={l.name} title={l.name} trailing={l.license} />)}
 </StSection>
 </>;
}
