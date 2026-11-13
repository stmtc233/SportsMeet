import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { IconCheck, IconChevronDown, iconStroke } from "./icons";
import type { StOption } from "./StFields";

interface PopoverProps {
  anchor: HTMLElement | null;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  /** Match the anchor's width at least (selects). */
  matchWidth?: boolean;
}

// The app's own popup layer: the whole menu fades in from the anchor's
// side with a 6px shift and a 0.96→1 scale. It aligns to the anchor's right
// edge when the anchor is in the right half, opens upwards when there is
// more room above, and scrolls inside when too tall.
function Popover({ anchor, open, onClose, children, matchWidth }: PopoverProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [shown, setShown] = useState(open);
  const [style, setStyle] = useState<CSSProperties>({});
  const [up, setUp] = useState(false);

  useEffect(() => {
    if (open) {
      setShown(true);
      return;
    }
    const t = setTimeout(() => setShown(false), 100);
    return () => clearTimeout(t);
  }, [open]);

  useLayoutEffect(() => {
    if (!open || !anchor) return;
    const r = anchor.getBoundingClientRect();
    const gap = 4;
    const below = window.innerHeight - r.bottom - gap - 8;
    const above = r.top - gap - 8;
    const wanted = ref.current?.scrollHeight ?? 0;
    const goUp = wanted > below && above > below;
    const next: CSSProperties = { maxHeight: Math.max(120, goUp ? above : below) };
    if (matchWidth) next.minWidth = r.width;
    if (r.left + r.width / 2 > window.innerWidth / 2) next.right = window.innerWidth - r.right;
    else next.left = r.left;
    if (goUp) next.bottom = window.innerHeight - r.top + gap;
    else next.top = r.bottom + gap;
    setUp(goUp);
    setStyle(next);
  }, [open, anchor, matchWidth, shown]);

  useEffect(() => {
    if (!open) return;
    const down = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!ref.current?.contains(t) && !anchor?.contains(t)) onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
        anchor?.focus();
      }
    };
    const away = () => onClose();
    document.addEventListener("mousedown", down, true);
    document.addEventListener("keydown", key, true);
    window.addEventListener("resize", away);
    return () => {
      document.removeEventListener("mousedown", down, true);
      document.removeEventListener("keydown", key, true);
      window.removeEventListener("resize", away);
    };
  }, [open, anchor, onClose]);

  if (!shown) return null;
  // Inside a modal dialog the popup must live in the dialog's top layer.
  const host = anchor?.closest("dialog") ?? document.body;
  return createPortal(
    <div ref={ref} className="st-popover" style={style} data-open={open || undefined} data-up={up || undefined} role="menu" inert={!open} aria-hidden={!open}>
      {children}
    </div>,
    host,
  );
}

export interface StMenuItem {
  label: ReactNode;
  onSelect: () => void;
  icon?: ReactNode;
  disabled?: boolean;
  danger?: boolean;
  /** Draws a separator above this item. */
  separated?: boolean;
}

/** An action menu opened from a trigger. */
export function StMenu({ items, trigger }: { items: StMenuItem[]; trigger: (props: { onClick: () => void; ref: (el: HTMLElement | null) => void; "aria-expanded": boolean }) => ReactNode }) {
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  return (
    <>
      {trigger({ onClick: () => setOpen((v) => !v), ref: setAnchor, "aria-expanded": open })}
      <Popover anchor={anchor} open={open} onClose={() => setOpen(false)}>
        {items.map((item, i) => (
          <button key={i} type="button" role="menuitem" className="st-menu-item" data-danger={item.danger || undefined} data-separated={item.separated || undefined} disabled={item.disabled}
            onClick={() => {
              setOpen(false);
              item.onSelect();
            }}>
            {item.icon && <span className="st-menu-item__icon">{item.icon}</span>}
            {item.label}
          </button>
        ))}
      </Popover>
    </>
  );
}

/** A dropdown choice; for 5 options or fewer prefer StSegmented. */
export function StSelect<T extends string>({ label, value, options, onChange, placeholder = "请选择", width = "auto", disabled, size = "medium", "aria-label": ariaLabel }: {
  label?: ReactNode;
  "aria-label"?: string;
  value: T | "";
  options: StOption<T>[];
  onChange: (v: T) => void;
  placeholder?: string;
  width?: "auto" | "narrow" | "fill";
  disabled?: boolean;
  size?: "small" | "medium";
}) {
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const id = useId();
  const current = options.find((o) => o.value === value);
  return (
    <div className={`st-field st-field--${width}`}>
      {label && (
        <label className="st-field__label" htmlFor={id}>
          {label}
        </label>
      )}
      <button id={id} aria-label={ariaLabel} ref={setAnchor} type="button" className={`st-select st-select--${size}`} aria-haspopup="listbox" aria-expanded={open} disabled={disabled} onClick={() => setOpen((v) => !v)}>
        <span className="st-select__value" data-placeholder={current ? undefined : true}>
          {current ? current.label : placeholder}
        </span>
        <IconChevronDown size={16} strokeWidth={iconStroke} />
      </button>
      <Popover anchor={anchor} open={open} onClose={() => setOpen(false)} matchWidth>
        {options.map((o) => (
          <button key={o.value} type="button" role="menuitemradio" aria-checked={o.value === value} className="st-menu-item" disabled={o.disabled}
            onClick={() => {
              setOpen(false);
              onChange(o.value);
            }}>
            <span className="st-menu-item__check">{o.value === value && <IconCheck size={16} strokeWidth={iconStroke} />}</span>
            {o.label}
          </button>
        ))}
      </Popover>
    </div>
  );
}

/**
 * A dropdown of several choices; the menu stays open while ticking. Nothing
 * ticked means every choice (shown as allLabel), and so does ticking them all.
 */
export function StMultiSelect<T extends string>({ value, options, onChange, allLabel, width = "auto", disabled, "aria-label": ariaLabel }: {
  value: T[];
  options: StOption<T>[];
  onChange: (v: T[]) => void;
  allLabel: string;
  "aria-label"?: string;
  width?: "auto" | "narrow" | "fill";
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const picked = options.filter((o) => value.includes(o.value));
  const toggle = (v: T) => {
    const next = value.includes(v) ? value.filter((x) => x !== v) : [...value, v];
    onChange(next.length === options.length ? [] : next);
  };
  const item = (key: string, label: ReactNode, checked: boolean, onClick: () => void, separated = false) => (
    <button key={key} type="button" role="menuitemcheckbox" aria-checked={checked} className="st-menu-item" data-separated={separated || undefined} onClick={onClick}>
      <span className="st-menu-item__check">{checked && <IconCheck size={16} strokeWidth={iconStroke} />}</span>
      {label}
    </button>
  );
  return (
    <div className={`st-field st-field--${width}`}>
      <button ref={setAnchor} type="button" aria-label={ariaLabel} className="st-select st-select--medium" aria-haspopup="menu" aria-expanded={open} disabled={disabled} onClick={() => setOpen((v) => !v)}>
        <span className="st-select__value">{picked.length ? picked.map((o) => o.label).join("、") : allLabel}</span>
        <IconChevronDown size={16} strokeWidth={iconStroke} />
      </button>
      <Popover anchor={anchor} open={open} onClose={() => setOpen(false)} matchWidth>
        {item("", allLabel, picked.length === 0, () => onChange([]))}
        {options.map((o, i) => item(o.value || "-", o.label, value.includes(o.value), () => toggle(o.value), i === 0))}
      </Popover>
    </div>
  );
}
