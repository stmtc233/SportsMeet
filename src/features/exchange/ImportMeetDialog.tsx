import { useEffect, useState } from "react";
import { StButton, StDialog, StNotice } from "../../design";
import { isStale } from "../../lib/errors";
import type { ImportPreview, PickedFile } from "../../mygo";
import { ExchangeController } from "./controller";

function tally(t: { new: number; reused: number }): string {
  return t.reused > 0 ? `新建 ${t.new}，复用 ${t.reused}` : `新建 ${t.new}`;
}

/** Previews a picked interchange file, then imports it on confirmation. meetId "" imports it as a new meet. */
export function ImportMeetDialog({ file, meetId, onClose, onImported }: { file: PickedFile | null; meetId: string; onClose: () => void; onImported?: (meetId: string) => void }) {
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    setPreview(null);
    if (!file) return;
    let live = true;
    void ExchangeController.preview(file.path, meetId).then((p) => {
      if (!live) return;
      if (p) setPreview(p);
      else onClose();
    });
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file, meetId]);

  const run = async () => {
    if (!file || !preview) return;
    setBusy(true);
    try {
      const done = await ExchangeController.import(file.path, meetId, preview.hash);
      if (done) {
        onClose();
        onImported?.(done.meetId);
      } else {
        // The data moved since the preview: show what the import would do now.
        const again = await ExchangeController.preview(file.path, meetId);
        if (again) setPreview(again);
      }
    } catch (err) {
      if (!isStale(err)) throw err;
    } finally {
      setBusy(false);
    }
  };

  const bad = (preview?.errors.length ?? 0) > 0;
  return (
    <StDialog open={file !== null} title={meetId ? "导入到本赛事" : "导入赛事文件"} onClose={onClose}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" disabled={!preview || bad} loading={busy} onClick={run}>导入</StButton>
        </>
      }>
      {!preview && <span>正在读取…</span>}
      {preview && bad && (
        <>
          <StNotice tone="danger">文件有误，无法导入</StNotice>
          <ul className="lines">
            {preview.errors.slice(0, 50).map((e, i) => (
              <li key={i}>{e}</li>
            ))}
          </ul>
        </>
      )}
      {preview && !bad && (
        <>
          <dl className="facts">
            <dt>赛事</dt>
            <dd>{preview.meetName}{preview.newMeet ? "（新建）" : ""}</dd>
            <dt>参赛单位</dt>
            <dd>{tally(preview.units)}</dd>
            <dt>运动员</dt>
            <dd>{tally(preview.athletes)}</dd>
            <dt>项目</dt>
            <dd>{tally(preview.events)}</dd>
            <dt>队伍</dt>
            <dd>{tally(preview.teams)}</dd>
            <dt>全能</dt>
            <dd>{tally(preview.combineds)}</dd>
            <dt>报名</dt>
            <dd>新增 {preview.entries}</dd>
            <dt>组</dt>
            <dd>新增 {preview.heats}</dd>
            <dt>成绩</dt>
            <dd>新增 {preview.resultsNew}，覆盖 {preview.resultsOverwritten}</dd>
          </dl>
          {preview.overwrites.length > 0 && (
            <>
              <StNotice tone="warning">将覆盖 {preview.overwrites.length} 条已有成绩</StNotice>
              <ul className="lines">
                {preview.overwrites.map((o, i) => (
                  <li key={i}>{o.athlete}：{o.before} → {o.after}</li>
                ))}
              </ul>
            </>
          )}
          {preview.warnings.length > 0 && (
            <ul className="lines">
              {preview.warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          )}
        </>
      )}
    </StDialog>
  );
}
