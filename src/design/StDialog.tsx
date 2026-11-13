import { useEffect, useId, useRef, type FormEvent, type ReactNode } from "react";
import { IconClose, iconStroke } from "./icons";
import { StIconButton } from "./StButton";
import { StFieldFeedback } from "./StFieldFeedback";

interface StDialogProps {
  open: boolean;
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  /** Buttons, right-aligned; the primary one last. */
  actions?: ReactNode;
  /** Runs when Enter is pressed in a field or a submit button is clicked. */
  onSubmit?: () => unknown | Promise<unknown>;
  size?: "small" | "medium" | "large";
}

/** Every dialog: a modal with a title, a body and a row of actions. */
export function StDialog({ open, title, onClose, children, actions, onSubmit, size = "medium" }: StDialogProps) {
  const titleId = useId();
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);
  if (!open) return null;
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const focusError = () => {
      const field = form.querySelector<HTMLElement>('[aria-invalid="true"]:not(:disabled)');
      field?.focus();
      return !!field;
    };
    if (focusError()) return;
    await onSubmit?.();
    // Custom validation can arrive with the next React commit.
    requestAnimationFrame(() => { if (form.isConnected) focusError(); });
  };
  return (
    <dialog ref={ref} aria-labelledby={titleId} className={`st-dialog st-dialog--${size}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}>
      <form className="st-dialog__form" onSubmit={submit}>
        <StFieldFeedback>
          <header className="st-dialog__head">
            <h2 id={titleId}>{title}</h2>
            <StIconButton label="关闭" onClick={onClose}>
              <IconClose size={18} strokeWidth={iconStroke} />
            </StIconButton>
          </header>
          <div className="st-dialog__body">{children}</div>
          {actions && <footer className="st-dialog__actions">{actions}</footer>}
        </StFieldFeedback>
      </form>
    </dialog>
  );
}
