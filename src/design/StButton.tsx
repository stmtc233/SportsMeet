import { useEffect, useRef, useState, type AnchorHTMLAttributes, type ButtonHTMLAttributes, type ReactNode } from "react";
import { IconChevronRight, iconStroke } from "./icons";

type Variant = "primary" | "secondary" | "tonal" | "danger" | "ghost";
type Size = "small" | "medium" | "large";

interface StButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className"> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  icon?: ReactNode;
  /** Every text this button may show; its width is that of the widest. */
  reserveWidthFor?: string[];
  /** A fold toggle: sets aria-expanded and shows a chevron that turns with it. */
  expanded?: boolean;
}

/** Every text button. */
export function StButton({ variant = "secondary", size = "medium", loading, icon, reserveWidthFor, expanded, children, disabled, type = "button", ...rest }: StButtonProps) {
  const disclosure = expanded !== undefined;
  const shown = disclosure ? <IconChevronRight size={size === "small" ? 16 : 18} strokeWidth={iconStroke} /> : icon;
  return (
    <button {...rest} type={type} disabled={disabled || loading} className={`st-button st-button--${variant} st-button--${size}`} data-loading={loading || undefined}
      aria-expanded={disclosure ? expanded : rest["aria-expanded"]} data-disclosure={disclosure || undefined}>
      {/* Shows only when the action takes a while: quick ones do not flash it. */}
      {loading && <span className="st-button__spinner" aria-hidden />}
      {shown && <span className="st-button__icon">{shown}</span>}
      <span className="st-button__label">
        <span>{children}</span>
        {reserveWidthFor?.map((text) => (
          <span key={text} className="st-button__ghost" aria-hidden>
            {text}
          </span>
        ))}
      </span>
    </button>
  );
}

/** A navigation link with the same appearance and focus treatment as buttons. */
export function StLinkButton({ variant = "secondary", size = "medium", children, ...rest }: Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "className" | "href"> & {
  href: string;
  variant?: Variant;
  size?: Size;
}) {
  return <a {...rest} className={`st-button st-button--${variant} st-button--${size}`}>{children}</a>;
}

interface StIconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className" | "aria-label"> {
  /** Accessible name; also the tooltip. */
  label: string;
  variant?: "plain" | "tonal" | "filled";
  size?: Size;
}

/** An icon-only button; the label is required. */
export function StIconButton({ label, variant = "plain", size = "medium", type = "button", children, ...rest }: StIconButtonProps) {
  return (
    <button {...rest} type={type} aria-label={label} title={label} className={`st-icon-button st-icon-button--${variant} st-icon-button--${size}`}>
      {children}
    </button>
  );
}

interface StConfirmButtonProps {
  children: ReactNode;
  onConfirm: () => void;
  disabled?: boolean;
  loading?: boolean;
  size?: Size;
  /** Shown red from the start (a draft that will delete results). */
  armedLook?: boolean;
}

const confirmText = "再按一次确认";

/** A dangerous action: the first click arms it, a second one within a few seconds runs it. */
export function StConfirmButton({ children, onConfirm, disabled, loading, size, armedLook }: StConfirmButtonProps) {
  const [armed, setArmed] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const click = () => {
    clearTimeout(timer.current);
    if (armed) {
      setArmed(false);
      onConfirm();
      return;
    }
    setArmed(true);
    timer.current = setTimeout(() => setArmed(false), 4000);
  };
  return (
    <StButton variant={armed || armedLook ? "danger" : "secondary"} size={size} disabled={disabled} loading={loading} onClick={click} reserveWidthFor={[confirmText]}>
      {armed ? confirmText : children}
    </StButton>
  );
}
