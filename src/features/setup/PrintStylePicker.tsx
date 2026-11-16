import { StButton, StRow, StSelect } from "../../design";
import { useOptimistic } from "../../lib/optimistic";
import { MeetStyleController, defaultStyleId, usePrintStyles } from "../settings/print-styles/controller";
import { useManageStyles } from "../settings/print-styles/open";

/** The print style of the meet's reports, and the way to 设置 → 打印样式. Takes effect at once. */
export function PrintStylePicker({ meetId, styleId }: { meetId: string; styleId: string }) {
  const { data: styles } = usePrintStyles();
  const manage = useManageStyles();
  // The meet stores "" for the default style.
  const { data, save } = useOptimistic({ styleId }, ({ styleId }) => MeetStyleController.set(meetId, styleId));
  const value = data?.styleId || defaultStyleId;
  const options = styles?.map((s) => ({ value: s.id, label: s.name })) ?? [];
  return (
    <div className="setup-fields setup-fields--row">
      <StRow>
        <StSelect label="打印样式" value={styles ? value : ""} options={options} onChange={(v) => save({ styleId: v === defaultStyleId ? "" : v })} />
        <StButton onClick={() => manage(value)}>管理样式</StButton>
      </StRow>
    </div>
  );
}
