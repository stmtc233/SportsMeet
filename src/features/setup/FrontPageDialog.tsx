import { useState } from "react";
import { StButton, StDialog, StMuted, StMultiSelect, StNotice, StStack, StTextArea, StTextField, toast } from "../../design";
import type { FrontPageView } from "../../mygo";
import { FrontPagesController, savedOf } from "../settings/print-styles/controller";
import { frontPageReports } from "./frontPages";

/** Adds (page undefined) or edits a front page: its title, the reports it goes in front of and its Markdown text. */
export function FrontPageDialog({ meetId, page, onClose }: { meetId: string; page?: FrontPageView; onClose: () => void }) {
  const [title, setTitle] = useState(page?.title ?? "");
  const [body, setBody] = useState(page?.bodyMd ?? "");
  const [reports, setReports] = useState<string[]>(page?.reports ?? []);
  // The page once saved, so that saving again changes it rather than adding another.
  const [id, setId] = useState(page?.id ?? "");
  const [warnings, setWarnings] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    const saved = savedOf(await FrontPagesController.save(meetId, id, { title, bodyMd: body, reports }));
    setBusy(false);
    if (!saved) return;
    setId(saved.page.id);
    setWarnings(saved.warnings);
    // Warnings stay in view: the text can be fixed and saved again.
    if (saved.warnings.length === 0) {
      toast("已保存", "success");
      onClose();
    }
  };
  return (
    <StDialog open size="large" title={page ? "编辑前置页" : "新建前置页"} onClose={() => !busy && onClose()} onSubmit={save}
      actions={
        <>
          <StButton disabled={busy} onClick={onClose}>{warnings.length > 0 ? "关闭" : "取消"}</StButton>
          <StButton variant="primary" loading={busy} type="submit">保存</StButton>
        </>
      }>
      <StStack gap={12}>
        {warnings.map((w) => <StNotice key={w} tone="warning">{w}</StNotice>)}
        <StTextField label="标题" autoFocus width="fill" maxLength={60} value={title} onChange={setTitle} disabled={busy} />
        <StStack gap={4}>
          <StMuted>印在</StMuted>
          <StMultiSelect aria-label="印在哪些报表前" width="fill" allLabel="全部报表" value={reports} options={frontPageReports} onChange={setReports} disabled={busy} />
        </StStack>
        <StTextArea label="正文" rows={14} value={body} onChange={setBody}
          hint="---pagebreak--- 分页，->文字<- 居中，![](asset:名称) 插入图片" />
      </StStack>
    </StDialog>
  );
}
