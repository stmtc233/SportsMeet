import { useEffect, useMemo, useState } from "react";
import { StButton, StDialog, StRow, StTable, StTag } from "../../design";
import type { EntryForm, EntryFormPreview } from "../../mygo";
import { EntriesController } from "./controller";

const actions: Record<string, { text: string; tone: "success" | "info" | "neutral" | "danger" }> = {
  create: { text: "新建", tone: "success" },
  update: { text: "更新", tone: "info" },
  keep: { text: "不变", tone: "neutral" },
  error: { text: "有误", tone: "danger" },
};

type Line = { key: string; unit: string; line: string; action: string; name: string; message: string };

/** Picked 报名登记表 → preview row by row → import. */
export function EntryFormDialog({ meetId, forms, onPick, onClose }: { meetId: string; forms: EntryForm[] | null; onPick: () => void; onClose: () => void }) {
  const [preview, setPreview] = useState<EntryFormPreview | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setPreview(null);
    if (!forms) return;
    let live = true;
    void EntriesController.previewForms(meetId, forms).then((p) => {
      if (live && p && typeof p === "object") setPreview(p);
    });
    return () => {
      live = false;
    };
  }, [meetId, forms]);

  const lines = useMemo(() => {
    const out: Line[] = [];
    preview?.forms.forEach((f, i) => {
      // A group's form names each row's unit.
      const unit = f.unit || f.group || f.file;
      if (f.problem) out.push({ key: `${i}`, unit, line: "", action: "error", name: f.file, message: f.problem });
      for (const r of f.rows) out.push({ key: `${i}:a:${r.line}`, unit: r.unit || unit, line: String(r.line), action: r.action, name: r.name, message: r.message });
      for (const r of f.teamRows) out.push({ key: `${i}:t:${r.line}`, unit: r.unit || unit, line: `接力 ${r.line}`, action: r.action, name: r.name, message: r.message });
    });
    return out;
  }, [preview]);

  const run = async () => {
    if (!forms) return;
    setBusy(true);
    const done = await EntriesController.importForms(meetId, forms);
    setBusy(false);
    if (done) onClose();
  };

  return (
    <StDialog open={forms !== null} title="导入报名登记表" size="large" onClose={onClose}
      actions={
        <>
          <StButton onClick={onPick}>重新选择</StButton>
          <StButton variant="primary" loading={busy} disabled={!preview || preview.errors > 0 || lines.length === 0} onClick={run}>导入</StButton>
        </>
      }>
      {preview && (
        <>
          <StRow>
            <StTag tone="success">新建 {preview.creates}</StTag>
            <StTag tone="info">更新 {preview.updates}</StTag>
            <StTag tone={preview.errors > 0 ? "danger" : "neutral"}>有误 {preview.errors}</StTag>
            {preview.entries > 0 && <StTag>报名 {preview.entries}</StTag>}
            {preview.teams > 0 && <StTag>队伍 {preview.teams}</StTag>}
          </StRow>
          <div className="preview-table view-enter">
            <StTable<Line> dense rows={lines} rowKey={(r) => r.key}
              columns={[
                { key: "unit", title: "单位", width: "minmax(96px, 1fr)", render: (r) => r.unit },
                { key: "line", title: "行", width: "72px", align: "end", render: (r) => r.line },
                { key: "action", title: "操作", width: "72px", render: (r) => <StTag tone={actions[r.action]?.tone}>{actions[r.action]?.text}</StTag> },
                { key: "name", title: "姓名 / 项目", width: "minmax(120px, 1.2fr)", render: (r) => r.name },
                { key: "message", title: "说明", width: "minmax(160px, 2fr)", render: (r) => r.message },
              ]} />
          </div>
        </>
      )}
    </StDialog>
  );
}
