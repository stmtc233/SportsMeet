import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { IconCheck, IconChevronDown, iconStroke } from "./icons";
import type { StOption } from "./StFields";

interface PopoverProps {
  id: string;
  role?: "menu" | "listbox";
  initialFocus: "first" | "last" | "selected" | null;
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
function Popover({ id, role = "menu", initialFocus, anchor, open, onClose, children, matchWidth }: PopoverProps) {
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

  useLayoutEffect(() => {
    if (!open || !initialFocus || !ref.current) return;
    const items = [...ref.current.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
    const selected = items.find((item) => item.getAttribute("aria-selected") === "true" || item.getAttribute("aria-checked") === "true");
    (initialFocus === "last" ? items.at(-1) : initialFocus === "selected" ? selected ?? items[0] : items[0])?.focus();
  }, [open, shown, initialFocus]);

  useEffect(() => {
    if (!open) return;
    const down = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!ref.current?.contains(t) && !anchor?.contains(t)) onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (!ref.current?.contains(e.target as Node) && !anchor?.contains(e.target as Node)) return;
      if (e.isComposing) return;
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onClose();
        anchor?.focus();
      } else if (e.key === "Tab") {
        // Portals sit at the end of the document. Resume native Tab order
        // from the trigger rather than stepping through every popup item.
        anchor?.focus();
        onClose();
      }
    };
    const focus = (e: FocusEvent) => {
      if (!ref.current?.contains(e.target as Node) && !anchor?.contains(e.target as Node)) onClose();
    };
    const away = () => onClose();
    document.addEventListener("mousedown", down, true);
    document.addEventListener("keydown", key, true);
    document.addEventListener("focusin", focus);
    window.addEventListener("resize", away);
    return () => {
      document.removeEventListener("mousedown", down, true);
      document.removeEventListener("keydown", key, true);
      document.removeEventListener("focusin", focus);
      window.removeEventListener("resize", away);
    };
  }, [open, anchor, onClose]);

  const search = useRef({ text: "", at: 0 });
  useEffect(() => { search.current = { text: "", at: 0 }; }, [open]);
  const navigate = (e: ReactKeyboardEvent<HTMLDivElement>) => {
    if (e.nativeEvent.isComposing || e.altKey || e.ctrlKey || e.metaKey) return;
    const items = [...e.currentTarget.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
    const index = items.indexOf(document.activeElement as HTMLButtonElement);
    let next: HTMLButtonElement | undefined;
    if (e.key === "ArrowDown") next = items[(index + 1) % items.length];
    else if (e.key === "ArrowUp") next = items[(index - 1 + items.length) % items.length];
    else if (e.key === "Home") next = items[0];
    else if (e.key === "End") next = items.at(-1);
    else if (e.key.length === 1 && e.key !== " ") {
      const now = performance.now();
      const text = (now - search.current.at < 700 ? search.current.text : "") + e.key.toLocaleLowerCase();
      search.current = { text, at: now };
      const prefix = [...text].every((c) => c === text[0]) ? text[0]! : text;
      next = [...items.slice(index + 1), ...items.slice(0, index + 1)].find((item) => item.textContent?.trim().toLocaleLowerCase().startsWith(prefix));
    } else return;
    e.preventDefault();
    e.stopPropagation();
    next?.focus();
  };

  if (!shown) return null;
  // Inside a modal dialog the popup must live in the dialog's top layer.
  const host = anchor?.closest("dialog") ?? document.body;
  return createPortal(
    <div id={id} ref={ref} className="st-popover" style={style} data-open={open || undefined} data-up={up || undefined} role={role} aria-labelledby={anchor?.id || undefined} inert={!open} aria-hidden={!open}
      onKeyDown={navigate} onFocus={(e) => {
        const target = e.target as HTMLElement;
        if (target instanceof HTMLButtonElement) {
          e.currentTarget.querySelectorAll<HTMLButtonElement>("button").forEach((item) => { item.tabIndex = item === target ? 0 : -1; });
        }
      }}>
      {children}
    </div>,
    host,
  );
}

// All popup triggers share their keyboard open path and focus restoration.
function usePopup(selection = false) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [initialFocus, setInitialFocus] = useState<PopoverProps["initialFocus"]>(null);
  const close = () => setOpen(false);
  const restore = () => { anchor?.focus({ preventScroll: true }); close(); };
  const triggerProps = {
    id: `${id}-trigger`, ref: setAnchor, "aria-expanded": open, "aria-controls": open ? id : undefined,
    onClick: (e: { detail: number }) => { setInitialFocus(e.detail === 0 ? selection ? "selected" : "first" : null); setOpen((v) => !v); },
    onKeyDown: (e: ReactKeyboardEvent<HTMLElement>) => {
      if (e.nativeEvent.isComposing || (e.key !== "ArrowDown" && e.key !== "ArrowUp")) return;
      e.preventDefault();
      setInitialFocus(selection ? "selected" : e.key === "ArrowUp" ? "last" : "first");
      setOpen(true);
    },
  };
  return { triggerProps, restore, popupProps: { id, open, anchor, initialFocus, onClose: close } };
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
export function StMenu({ items, trigger }: { items: StMenuItem[]; trigger: (props: ReturnType<typeof usePopup>["triggerProps"] & { "aria-haspopup": "menu" }) => ReactNode }) {
  const popup = usePopup();
  return (
    <>
      {trigger({ ...popup.triggerProps, "aria-haspopup": "menu" })}
      <Popover {...popup.popupProps}>
        {items.map((item, i) => (
          <button key={i} type="button" role="menuitem" tabIndex={-1} className="st-menu-item" data-danger={item.danger || undefined} data-separated={item.separated || undefined} disabled={item.disabled}
            onClick={() => {
              popup.restore();
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
  const popup = usePopup(true);
  const id = popup.triggerProps.id;
  const current = options.find((o) => o.value === value);
  return (
    <div className={`st-field st-field--${width}`}>
      {label && (
        <label className="st-field__label" htmlFor={id}>
          {label}
        </label>
      )}
      <button {...popup.triggerProps} aria-label={ariaLabel} type="button" className={`st-select st-select--${size}`} aria-haspopup="listbox" disabled={disabled}>
        <span className="st-select__value" data-placeholder={current ? undefined : true}>
          {current ? current.label : placeholder}
        </span>
        <IconChevronDown size={16} strokeWidth={iconStroke} aria-hidden />
      </button>
      <Popover {...popup.popupProps} role="listbox" matchWidth>
        {options.map((o) => (
          <button key={o.value} type="button" role="option" aria-selected={o.value === value} tabIndex={-1} className="st-menu-item" disabled={o.disabled}
            onClick={() => {
              popup.restore();
              onChange(o.value);
            }}>
            <span className="st-menu-item__check" aria-hidden>{o.value === value && <IconCheck size={16} strokeWidth={iconStroke} />}</span>
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
  const popup = usePopup(true);
  const picked = options.filter((o) => value.includes(o.value));
  const toggle = (v: T) => {
    const next = value.includes(v) ? value.filter((x) => x !== v) : [...value, v];
    onChange(next.length === options.length ? [] : next);
  };
  const item = (key: string, label: ReactNode, checked: boolean, onClick: () => void, separated = false, disabled = false) => (
    <button key={key} type="button" role="menuitemcheckbox" tabIndex={-1} aria-checked={checked} className="st-menu-item" data-separated={separated || undefined} disabled={disabled} onClick={onClick}>
      <span className="st-menu-item__check" aria-hidden>{checked && <IconCheck size={16} strokeWidth={iconStroke} />}</span>
      {label}
    </button>
  );
  return (
    <div className={`st-field st-field--${width}`}>
      <button {...popup.triggerProps} type="button" aria-label={ariaLabel} className="st-select st-select--medium" aria-haspopup="menu" disabled={disabled}>
        <span className="st-select__value">{picked.length ? picked.map((o) => o.label).join("、") : allLabel}</span>
        <IconChevronDown size={16} strokeWidth={iconStroke} aria-hidden />
      </button>
      <Popover {...popup.popupProps} matchWidth>
        {item("", allLabel, picked.length === 0, () => onChange([]))}
        {options.map((o, i) => item(o.value || "-", o.label, value.includes(o.value), () => toggle(o.value), i === 0, o.disabled))}
      </Popover>
    </div>
  );
}
