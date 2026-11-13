import { useState } from "react";
import { lastMeetTab, useNav } from "../../app/nav";
import { IconAdd, IconDelete, IconEdit, IconImport, IconSettings, IconMeet, StButton, StCard, StEmpty, StIconButton, StListItem, StNotice, StPage, StToolbar, iconStroke } from "../../design";
import type { Meet, MeetCounts, PickedFile } from "../../mygo";
import { ImportMeetDialog } from "../exchange/ImportMeetDialog";
import { MeetsController } from "./controller";
import { useMeets } from "./data";
import { DeleteDialog } from "./DeleteDialog";
import { MeetDialog } from "./MeetDialog";

export function MeetsPage() {
  const { go } = useNav();
  const { data: meets, error } = useMeets();
  const [editing, setEditing] = useState<{ meet?: Meet } | null>(null);
  const [deleting, setDeleting] = useState<{ meet: Meet; counts: MeetCounts } | null>(null);
  const [file, setFile] = useState<PickedFile | null>(null);
  const open = (meetId: string) => go({ page: "meet", meetId, tab: lastMeetTab(meetId) });

  const askDelete = async (meet: Meet) => {
    const counts = await MeetsController.count(meet.id);
    if (counts) setDeleting({ meet, counts });
  };
  const pick = async () => {
    const picked = await MeetsController.pickFile();
    if (picked?.path) setFile(picked);
  };

  return (
    <StPage scrollKey="meets"
      toolbar={
        <StToolbar title="赛事">
          <StIconButton label="设置" onClick={() => go({ page: "settings", section: null })}>
            <IconSettings size={18} strokeWidth={iconStroke} />
          </StIconButton>
          <StButton icon={<IconImport size={16} strokeWidth={iconStroke} />} onClick={pick}>导入赛事文件</StButton>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>新建赛事</StButton>
        </StToolbar>
      }>
      <div className="narrow">
        {error && <StNotice tone="danger">{error}</StNotice>}
        <StCard padded={false}>
          {meets?.length === 0 && <StEmpty action={<StButton variant="primary" onClick={() => setEditing({})}>新建赛事</StButton>}>还没有赛事</StEmpty>}
          {(meets ?? []).map((m) => (
            <StListItem key={m.id} title={m.name} subtitle={[m.datesText, m.location].filter(Boolean).join(" · ")}
              leading={<IconMeet size={20} strokeWidth={iconStroke} />} onClick={() => open(m.id)}
              trailing={
                <span onClick={(e) => e.stopPropagation()} className="cell-actions">
                  <StIconButton label="编辑" onClick={() => setEditing({ meet: m })}>
                    <IconEdit size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                  <StIconButton label="删除" onClick={() => askDelete(m)}>
                    <IconDelete size={16} strokeWidth={iconStroke} />
                  </StIconButton>
                </span>
              } />
          ))}
        </StCard>
      </div>
      <MeetDialog open={editing !== null} meet={editing?.meet} onClose={() => setEditing(null)} onSaved={editing?.meet ? undefined : open} />
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.meet.name ?? ""}」？`} onClose={() => setDeleting(null)} onDelete={() => MeetsController.remove(deleting!.meet.id)}>
        将删除 {deleting?.counts.events} 个项目、{deleting?.counts.athletes} 名运动员与 {deleting?.counts.results} 条成绩，无法撤销。
      </DeleteDialog>
      <ImportMeetDialog file={file} meetId="" onClose={() => setFile(null)} onImported={open} />
    </StPage>
  );
}
