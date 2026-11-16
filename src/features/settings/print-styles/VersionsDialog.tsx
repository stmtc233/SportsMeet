import { useState } from "react";
import { StButton, StDialog, StListItem, StNotice, StTag } from "../../../design";
import type { PrintStyleVersion } from "../../../mygo";
import { PrintStylesController, usePrintStyleVersions } from "./controller";

const sources: Record<string, string> = { ui: "界面", mcp: "AI", system: "系统" };

/** The kept versions of a style, newest first; restoring one is a save of its own. */
export function VersionsDialog({ id, open, onClose }: { id: string; open: boolean; onClose: () => void }) {
  const { data, error } = usePrintStyleVersions(id, open);
  const [busy, setBusy] = useState<number | null>(null);
  const restore = async (v: PrintStyleVersion) => {
    setBusy(v.version);
    const ok = await PrintStylesController.restore(id, v.version);
    setBusy(null);
    if (ok) onClose();
  };
  return (
    <StDialog open={open} title="历史版本" onClose={onClose} actions={<StButton onClick={onClose}>关闭</StButton>}>
      {error && <StNotice tone="danger">{error}</StNotice>}
      <div className="versions">
        {data?.map((v) => (
          <StListItem key={v.version} title={`${v.atText} · ${sources[v.source] ?? v.source}`} subtitle={v.reason}
            trailing={v.current ? <StTag tone="info">当前</StTag> : <StButton size="small" loading={busy === v.version} disabled={busy !== null} onClick={() => void restore(v)}>恢复</StButton>} />
        ))}
      </div>
    </StDialog>
  );
}
