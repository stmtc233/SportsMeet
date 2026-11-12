import { useState } from "react";
import { StNotice, StSection, StSegmented, StSwitchItem } from "../../design";
import { SettingsController, usePreferences } from "./controller";

export function PreferencesPanel({ section }: { section: "general" | "arrange" | "print" }) {
  const { data, error } = usePreferences();
  const [busy, setBusy] = useState(false);
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  const save = async (patch: Partial<typeof data>) => {
    if (busy) return;
    setBusy(true);
    await SettingsController.save({ ...data, ...patch });
    setBusy(false);
  };
  if (section === "general") return (
    <StSection title="外观">
      <StSegmented value={data.theme} onChange={(theme) => void save({ theme })}
        options={[{ value: "system", label: "跟随系统", disabled: busy }, { value: "light", label: "浅色", disabled: busy }, { value: "dark", label: "深色", disabled: busy }]} />
    </StSection>
  );
  if (section === "arrange") return (
    <StSection title="自动编排默认值" padded={false}>
      <StSwitchItem title="同单位分散" subtitle="同单位尽量不同组" checked={data.separateUnits} disabled={busy} onChange={(separateUnits) => void save({ separateUnits })} />
      <StSwitchItem title="均衡人数" checked={data.balance} disabled={busy} onChange={(balance) => void save({ balance })} />
    </StSection>
  );
  return (
    <StSection title="分页" padded={false}>
      <StSwitchItem title="每个项目另起一页" checked={data.printNewPage} disabled={busy} onChange={(printNewPage) => void save({ printNewPage })} />
    </StSection>
  );
}
