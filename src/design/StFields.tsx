import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type InputHTMLAttributes, type KeyboardEvent, type ReactNode, type Ref } from "react";
import { isCallError } from "mygo-runtime";
import type { Event, EventKind } from "../mygo";
import { normalizeMark } from "../lib/marks";
import { IconCheck, IconSearch, iconStroke } from "./icons";
import { useFieldFeedback } from "./StFieldFeedback";

interface FieldShellProps {
  label?: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  width?: "auto" | "narrow" | "mark" | "fill";
  id: string;
  group?: boolean;
  hideError?: boolean;
  children: ReactNode;
}

// The label sits above the box, hint and error below it.
function FieldShell({ label, hint, error, width = "auto", id, group, hideError, children }: FieldShellProps) {
  const ref = useRef<HTMLDivElement>(null);
  const announce = useFieldFeedback();
  const message = () => {
    const el = ref.current;
    const field = el?.querySelector<HTMLInputElement>("input, textarea");
    const name = el?.querySelector("label")?.textContent || field?.getAttribute("aria-label") || "";
    const text = el?.querySelector(`#${CSS.escape(id)}-error`)?.textContent;
    announce(id, text ? `${name}：${text}` : "");
  };
  const mounted = useRef(false);
  useEffect(() => {
    if (mounted.current) {
      if (!error) announce(id, "");
      else if (!ref.current?.contains(document.activeElement)) message();
    }
    mounted.current = true;
  }, [error, announce, id]);
  useEffect(() => () => announce(id, ""), [announce, id]);
  return (
    <div ref={ref} className={`st-field st-field--${width}`} onFocus={() => announce(id, "")} onBlur={(e) => {
      if (error && !e.currentTarget.contains(e.relatedTarget as Node | null)) message();
    }}>
      {label && (
        group ? <span className="st-field__label" id={`${id}-label`}>{label}</span> : <label className="st-field__label" htmlFor={id}>{label}</label>
      )}
      {children}
      {error ? <span className={hideError ? "st-sr-only" : "st-field__error"} id={`${id}-error`}>{error}</span> : hint ? <span className="st-field__hint" id={`${id}-hint`}>{hint}</span> : null}
    </div>
  );
}

function describedBy(id: string, error?: ReactNode, hint?: ReactNode, external?: string) {
  return [external, error ? `${id}-error` : hint ? `${id}-hint` : undefined].filter(Boolean).join(" ") || undefined;
}

interface StTextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "value" | "prefix" | "className" | "width"> {
  label?: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  value: string;
  onChange: (value: string) => void;
  prefix?: ReactNode;
  suffix?: ReactNode;
  width?: FieldShellProps["width"];
  align?: "start" | "end";
  ref?: Ref<HTMLInputElement>;
  /** Marks the box invalid without a message below it. */
  invalid?: boolean;
}

export function StTextField({ label, hint, error, invalid, value, onChange, prefix, suffix, width, align, id, ...rest }: StTextFieldProps) {
  const auto = useId();
  const fieldId = id ?? auto;
  return (
    <FieldShell label={label} hint={hint} error={error} width={width} id={fieldId}>
      <span className="st-input" data-invalid={error || invalid ? true : undefined}>
        {prefix && <span className="st-input__affix">{prefix}</span>}
        <input {...rest} id={fieldId} value={value} onChange={(e) => onChange(e.target.value)} autoComplete="off" spellCheck={false} data-align={align}
          aria-invalid={error || invalid ? true : rest["aria-invalid"] ?? false} aria-describedby={describedBy(fieldId, error, hint, rest["aria-describedby"])} />
        {suffix && <span className="st-input__affix">{suffix}</span>}
      </span>
    </FieldShell>
  );
}

/** Height trials are text in a narrow cell: the heat's Go preview marks it
 * invalid and the page shows the message once for the row, linked through
 * aria-describedby. */
export function StTrialsField({ invalid, ...props }: Omit<StTextFieldProps, "inputMode" | "align" | "width" | "error" | "hint" | "label">) {
  return <StTextField {...props} invalid={invalid} inputMode="text" align="end" width="fill" aria-invalid={invalid || undefined} />;
}

export function StSearchField({ value, onChange, placeholder = "搜索", width = "auto" }: { value: string; onChange: (v: string) => void; placeholder?: string; width?: FieldShellProps["width"] }) {
  return <StTextField value={value} onChange={onChange} placeholder={placeholder} width={width} aria-label={placeholder} prefix={<IconSearch size={16} strokeWidth={iconStroke} />} />;
}

/** A multi-line box, for pasted tables. */
export function StTextArea({ label, hint, value, onChange, rows = 8, placeholder }: { label?: ReactNode; hint?: ReactNode; value: string; onChange: (v: string) => void; rows?: number; placeholder?: string }) {
  const id = useId();
  return (
    <FieldShell label={label} hint={hint} width="fill" id={id}>
      <textarea id={id} className="st-textarea" rows={rows} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} spellCheck={false} aria-describedby={describedBy(id, undefined, hint)} />
    </FieldShell>
  );
}

interface StMarkFieldProps {
 event?: Event;
  kind: EventKind;
  label?: ReactNode;
  value: string;
  onChange: (value: string) => void;
  /** Called with the tidied text once what was typed parses. */
  onCommit?: (value: string) => void;
  disabled?: boolean;
  placeholder?: string;
  width?: FieldShellProps["width"];
  /** Other texts accepted as they are (X and - for attempts). */
  accept?: (text: string) => boolean;
  onKeyDown?: (e: KeyboardEvent<HTMLInputElement>) => void;
  "aria-label"?: string;
  name?: string;
}

/**
 * A mark as typed (12.34, 1:05.67, 6.52). Parsing and formatting are Go's
 * (scoring), asked on blur; this component has no parser of its own.
 */
export function StMarkField({ event, kind, label, value, onChange, onCommit, disabled, placeholder, width = "mark", accept, onKeyDown, name, ...rest }: StMarkFieldProps) {
  const [error, setError] = useState("");
  const id = useId();
  useEffect(() => {
    if (value === "") setError("");
  }, [value]);
  const blur = async () => {
    const text = value.trim();
    if (text === "" || accept?.(text)) {
      setError("");
      if (text !== value) onChange(text);
      onCommit?.(text);
      return;
    }
    try {
      const tidy = await normalizeMark(kind, text, event);
      setError("");
      if (tidy !== value) onChange(tidy);
      onCommit?.(tidy);
    } catch (err) {
      setError(isCallError(err) ? err.message : "成绩格式不正确");
    }
  };
  return (
    <FieldShell label={label} error={error} hideError={!label} width={width} id={id}>
      <span className="st-input st-input--mark" data-invalid={error ? true : undefined} title={error || undefined}>
        <input id={id} name={name} value={value} disabled={disabled} placeholder={placeholder} inputMode="decimal" autoComplete="off" spellCheck={false} data-align="end"
          aria-label={rest["aria-label"]} aria-invalid={!!error} aria-describedby={describedBy(id, error)}
          onChange={(e) => { setError(""); onChange(e.target.value); }} onBlur={blur} onKeyDown={onKeyDown} />
      </span>
    </FieldShell>
  );
}

/** A local date (YYYY-MM-DD). */
export function StDateField({ label, value, onChange, error, disabled }: { label?: ReactNode; value: string; onChange: (v: string) => void; error?: ReactNode; disabled?: boolean }) {
  const id = useId();
  return (
    <FieldShell label={label} error={error} id={id}>
      <span className="st-input" data-invalid={error ? true : undefined}>
        <input id={id} type="date" disabled={disabled} value={value} onChange={(e) => onChange(e.target.value)} aria-invalid={!!error} aria-describedby={describedBy(id, error)} />
      </span>
    </FieldShell>
  );
}

/** A 24-hour time of day (HH:mm). */
export function StTimeField({ label, value, onChange, disabled }: { label?: ReactNode; value: string; onChange: (v: string) => void; disabled?: boolean }) {
  const id = useId();
  return (
    <FieldShell label={label} id={id}>
      <span className="st-input">
        <input id={id} type="time" disabled={disabled} value={value} onChange={(e) => onChange(e.target.value)} />
      </span>
    </FieldShell>
  );
}

/** A check mark; inside a clickable row pass no onChange and let the row handle the click. */
export function StCheckbox({ checked, onChange, label, disabled, indeterminate, "aria-label": ariaLabel }: { checked: boolean; onChange?: (v: boolean) => void; label?: ReactNode; disabled?: boolean; indeterminate?: boolean; "aria-label"?: string }) {
  const input = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => { if (input.current) input.current.indeterminate = !!indeterminate && !checked; }, [indeterminate, checked]);
  const box = (
    <span className="st-checkbox" data-checked={checked || indeterminate || undefined} aria-hidden>
      {checked && <IconCheck size={14} strokeWidth={3} />}
      {!checked && indeterminate && <span className="st-checkbox__dash" />}
    </span>
  );
  if (!onChange) return box;
  return (
    <label className="st-check" data-disabled={disabled || undefined}>
      <input ref={input} type="checkbox" aria-label={ariaLabel} checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      {box}
      {label && <span>{label}</span>}
    </label>
  );
}

/** A switch; display only. Use StSwitchItem for a clickable row. */
export function StSwitch({ checked }: { checked: boolean }) {
  return <span className="st-switch" data-checked={checked || undefined} aria-hidden />;
}

/** A row with a switch on the right; the whole row toggles it. */
export function StSwitchItem({ title, subtitle, checked, onChange, disabled }: { title: ReactNode; subtitle?: ReactNode; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button type="button" role="switch" aria-checked={checked} className="st-list-item st-list-item--action" disabled={disabled} onClick={() => onChange(!checked)}>
      <span className="st-list-item__text">
        <span className="st-list-item__title">{title}</span>
        {subtitle && <span className="st-list-item__subtitle">{subtitle}</span>}
      </span>
      <span className="st-list-item__trailing">
        <StSwitch checked={checked} />
      </span>
    </button>
  );
}

export interface StOption<T extends string> {
  value: T;
  label: ReactNode;
  disabled?: boolean;
}

/** 2–5 exclusive options, or in-page tabs. */
export function StSegmented<T extends string>({ label, value, options, onChange, size = "medium", "aria-label": ariaLabel }: { label?: ReactNode; value: T; options: StOption<T>[]; onChange: (v: T) => void; size?: "small" | "medium"; "aria-label"?: string }) {
  const id = useId();
  const thumb = useThumb(value, options.map((o) => o.value).join("\n"));
  const tabValue = (options.find((o) => o.value === value && !o.disabled) ?? options.find((o) => !o.disabled))?.value;
  return (
    <FieldShell label={label} id={id} group>
      <div id={id} ref={thumb.ref} className={`st-segmented st-segmented--${size}`} role="radiogroup" aria-labelledby={label ? `${id}-label` : undefined} aria-label={label ? undefined : ariaLabel} data-thumb={thumb.style ? true : undefined} data-animate={thumb.animate || undefined}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing || e.altKey || e.ctrlKey || e.metaKey) return;
          const buttons = [...e.currentTarget.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
          const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
          if (i < 0) return;
          let next: HTMLButtonElement | undefined;
          if (e.key === "ArrowRight" || e.key === "ArrowDown") next = buttons[(i + 1) % buttons.length];
          else if (e.key === "ArrowLeft" || e.key === "ArrowUp") next = buttons[(i - 1 + buttons.length) % buttons.length];
          else if (e.key === "Home") next = buttons[0];
          else if (e.key === "End") next = buttons.at(-1);
          else return;
          e.preventDefault();
          next?.focus();
          next?.click();
        }}>
        {thumb.style && <span className="st-segmented__thumb" style={thumb.style} aria-hidden />}
        {options.map((o) => (
          <button key={o.value} type="button" role="radio" aria-checked={o.value === value} tabIndex={o.value === tabValue ? 0 : -1} disabled={o.disabled} onClick={() => onChange(o.value)}>
            {o.label}
          </button>
        ))}
      </div>
    </FieldShell>
  );
}

// The sliding background under the selected option. It is placed without
// a transition first, and slides from then on; while the control is not
// laid out (a hidden panel) the selected button paints its own background.
function useThumb(value: string, options: string) {
  const ref = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState<{ x: number; w: number } | null>(null);
  const [animate, setAnimate] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      const on = el.querySelector<HTMLElement>('button[aria-checked="true"]');
      const next = on && on.offsetWidth > 0 ? { x: on.offsetLeft, w: on.offsetWidth } : null;
      setBox((old) => (old?.x === next?.x && old?.w === next?.w ? old : next));
    };
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(el);
    el.querySelectorAll("button").forEach((b) => watch.observe(b));
    return () => watch.disconnect();
  }, [value, options]);
  const ready = box !== null;
  useEffect(() => {
    if (!ready) {
      setAnimate(false);
      return;
    }
    const frame = requestAnimationFrame(() => setAnimate(true));
    return () => cancelAnimationFrame(frame);
  }, [ready]);
  const style = box ? ({ "--thumb-x": `${box.x}px`, "--thumb-w": `${box.w}px` } as CSSProperties) : undefined;
  return { ref, style, animate };
}
