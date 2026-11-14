import { StNotice, StSection, StSegmented } from "../../design";
import { useOptimisticPreferences } from "./controller";

/** 通用: the app's look. A meet's arrange and print settings are on its 赛事设置. */
export function PreferencesPanel() {
  const { data, error, save } = useOptimisticPreferences();
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  return (
    <StSection title="外观">
      <StSegmented aria-label="外观" value={data.theme} onChange={(theme) => save({ theme })}
        options={[{ value: "system", label: "跟随系统" }, { value: "light", label: "浅色" }, { value: "dark", label: "深色" }]} />
    </StSection>
  );
}
