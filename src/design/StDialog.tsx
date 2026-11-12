import { useEffect, useRef, type FormEvent, type ReactNode } from "react";
import { IconClose, iconStroke } from "./icons";
import { StIconButton } from "./StButton";

interface StDialogProps {
  open: boolean;
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  /** Buttons, right-aligned; the primary one last. */
  actions?: ReactNode;
  /** Runs when Enter is pressed in a field or a submit button is clicked. */
  onSubmit?: () => void;
  size?: "small" | "medium" | "large";
}

/** Every dialog: a modal with a title, a body and a row of actions. */
export function StDialog({ open, title, onClose, children, actions, onSubmit, size = "medium" }: StDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);
  if (!open) return null;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit?.();
  };
  return (
    <dialog ref={ref} className={`st-dialog st-dialog--${size}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}>
      <form className="st-dialog__form" onSubmit={submit}>
        <header className="st-dialog__head">
          <h2>{title}</h2>
          <StIconButton label="关闭" onClick={onClose}>
            <IconClose size={18} strokeWidth={iconStroke} />
          </StIconButton>
        </header>
        <div className="st-dialog__body">{children}</div>
        {actions && <footer className="st-dialog__actions">{actions}</footer>}
      </form>
    </dialog>
  );
}
