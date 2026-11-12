import { useState, type ReactNode } from "react";
import { StButton, StDialog } from "../../design";

/** Confirms a deletion, saying what goes with it. */
export function DeleteDialog({ open, title, children, onClose, onDelete }: { open: boolean; title: string; children: ReactNode; onClose: () => void; onDelete: () => Promise<unknown> }) {
  const [busy, setBusy] = useState(false);
  const run = async () => {
    setBusy(true);
    const ok = await onDelete();
    setBusy(false);
    if (ok) onClose();
  };
  return (
    <StDialog open={open} title={title} size="small" onClose={onClose}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="danger" loading={busy} onClick={run}>删除</StButton>
        </>
      }>
      {children}
    </StDialog>
  );
}
