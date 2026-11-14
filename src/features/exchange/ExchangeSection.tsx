import { useState } from "react";
import { IconExport, IconImport, StButton, StListItem, StSection, iconStroke, toast } from "../../design";
import type { PickedFile } from "../../mygo";
import { ExchangeController } from "./controller";
import { ImportMeetDialog } from "./ImportMeetDialog";

/** 赛事文件: the whole meet out to a file, or a file's data into this meet. */
export function ExchangeSection({ meetId }: { meetId: string }) {
  const [file, setFile] = useState<PickedFile | null>(null);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    const out = await ExchangeController.export(meetId);
    setBusy(false);
    if (out === "saved") toast("已导出", "success");
  };
  const pick = async () => {
    const picked = await ExchangeController.pick();
    if (picked?.path) setFile(picked);
  };
  return (
    <StSection title="赛事文件" padded={false}>
      <StListItem title="导出赛事文件" subtitle="含报名、编排与成绩" leading={<IconExport size={20} strokeWidth={iconStroke} />}
        trailing={<StButton loading={busy} onClick={save}>导出</StButton>} />
      <StListItem title="导入到本赛事" leading={<IconImport size={20} strokeWidth={iconStroke} />}
        trailing={<StButton onClick={pick}>选择文件</StButton>} />
      <ImportMeetDialog file={file} meetId={meetId} onClose={() => setFile(null)} />
    </StSection>
  );
}
