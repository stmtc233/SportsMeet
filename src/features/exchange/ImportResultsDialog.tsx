import { useEffect, useState } from "react";
import { StButton, StDialog, StNotice } from "../../design";
import { isStale } from "../../lib/errors";
import type { PickedFile, ResultsImportPreview } from "../../mygo";
import { ExchangeController } from "./controller";

/** Previews a picked results file, then writes its marks into the meet on confirmation. */
export function ImportResultsDialog({ file, meetId, onClose }: { file: PickedFile | null; meetId: string; onClose: () => void }) {
  const [preview, setPreview] = useState<ResultsImportPreview | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    setPreview(null);
    if (!file) return;
    let live = true;
    void ExchangeController.previewResults(file.path, meetId).then((p) => {
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
      const done = await ExchangeController.importResults(file.path, meetId, preview.hash);
      if (done) {
        onClose();
      } else {
        // The meet moved since the preview: show what the import would do now.
        const again = await ExchangeController.previewResults(file.path, meetId);
        if (again) setPreview(again);
      }
    } catch (err) {
      if (!isStale(err)) throw err;
    } finally {
      setBusy(false);
    }
  };

  const bad = (preview?.errors.length ?? 0) > 0;
  const empty = preview !== null && preview.resultsNew + preview.resultsOverwritten === 0;
  return (
    <StDialog open={file !== null} title="导入成绩文件" onClose={onClose}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" disabled={!preview || bad || empty} loading={busy} onClick={run}>导入</StButton>
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
          <dl className="facts view-enter">
            <dt>组</dt>
            <dd>{preview.heats}</dd>
            <dt>成绩</dt>
            <dd>新增 {preview.resultsNew}，覆盖 {preview.resultsOverwritten}，未变 {preview.resultsUnchanged}</dd>
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
