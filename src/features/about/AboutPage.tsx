import { useEffect, useState } from "react";
import { useNav } from "../../app/nav";
import { IconBack, StIconButton, StListItem, StPage, StSection, StToolbar, iconStroke } from "../../design";
import { App } from "../../mygo";

// Open-source software the app ships with; keep in step with go.mod and
// package.json (docs/agents/design-system.md「图标」).
const licenses = [
  { name: "mygo", license: "MIT" },
  { name: "SQLite", license: "Public Domain" },
  { name: "React", license: "MIT" },
  { name: "Lucide", license: "ISC" },
  { name: "purego", license: "Apache-2.0" },
  { name: "golang.org/x/net、golang.org/x/text", license: "BSD-3-Clause" },
];

export function AboutPage() {
  const { go } = useNav();
  const [version, setVersion] = useState("");
  useEffect(() => {
    void App.info().then((i) => setVersion(i.version));
  }, []);
  return (
    <StPage
      toolbar={
        <StToolbar title="关于"
          leading={
            <StIconButton label="返回" onClick={() => go({ page: "meets" })}>
              <IconBack size={18} strokeWidth={iconStroke} />
            </StIconButton>
          } />
      }>
      <div className="narrow">
        <StSection title="SportsMeet" padded={false}>
          <StListItem title="版本" trailing={version} />
          <StListItem title="数据保存在本机" subtitle="不联网，不上传" />
        </StSection>
        <StSection title="开源许可" padded={false}>
          {licenses.map((l) => (
            <StListItem key={l.name} title={l.name} trailing={l.license} />
          ))}
        </StSection>
      </div>
    </StPage>
  );
}
