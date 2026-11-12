import { useEffect, useId, useState, type InputHTMLAttributes, type KeyboardEvent, type ReactNode } from "react";
import { isCallError } from "mygo-runtime";
import type { Event, EventKind } from "../mygo";
import { normalizeMark } from "../lib/marks";
import { IconCheck, IconSearch, iconStroke } from "./icons";

interface FieldShellProps {
  label?: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  width?: "auto" | "narrow" | "mark" | "fill";
  id: string;
  children: ReactNode;
}

// The label sits above the box, hint and error below it.
function FieldShell({ label, hint, error, width = "auto", id, children }: FieldShellProps) {
  return (
    <div className={`st-field st-field--${width}`}>
      {label && (
        <label className="st-field__label" htmlFor={id}>
          {label}
        </label>
      )}
      {children}
      {error ? <span className="st-field__error">{error}</span> : hint ? <span className="st-field__hint">{hint}</span> : null}
    </div>
  );
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
}

export function StTextField({ label, hint, error, value, onChange, prefix, suffix, width, align, id, ...rest }: StTextFieldProps) {
  const auto = useId();
  const fieldId = id ?? auto;
  return (
    <FieldShell label={label} hint={hint} error={error} width={width} id={fieldId}>
      <span className="st-input" data-invalid={error ? true : undefined}>
        {prefix && <span className="st-input__affix">{prefix}</span>}
        <input {...rest} id={fieldId} value={value} onChange={(e) => onChange(e.target.value)} autoComplete="off" spellCheck={false} data-align={align} />
        {suffix && <span className="st-input__affix">{suffix}</span>}
      </span>
    </FieldShell>
  );
}

export function StSearchField({ value, onChange, placeholder = "搜索", width = "auto" }: { value: string; onChange: (v: string) => void; placeholder?: string; width?: FieldShellProps["width"] }) {
  return <StTextField value={value} onChange={onChange} placeholder={placeholder} width={width} aria-label={placeholder} prefix={<IconSearch size={16} strokeWidth={iconStroke} />} />;
}

/** A multi-line box, for pasted tables. */
export function StTextArea({ label, hint, value, onChange, rows = 8, placeholder }: { label?: ReactNode; hint?: ReactNode; value: string; onChange: (v: string) => void; rows?: number; placeholder?: string }) {
  const id = useId();
  return (
    <FieldShell label={label} hint={hint} width="fill" id={id}>
      <textarea id={id} className="st-textarea" rows={rows} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} spellCheck={false} />
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
    <FieldShell label={label} error={label ? error : undefined} width={width} id={id}>
      <span className="st-input st-input--mark" data-invalid={error ? true : undefined} title={error || undefined}>
        <input id={id} name={name} value={value} disabled={disabled} placeholder={placeholder} inputMode="decimal" autoComplete="off" spellCheck={false} data-align="end"
          aria-label={rest["aria-label"]} aria-invalid={error ? true : undefined}
          onChange={(e) => onChange(e.target.value)} onBlur={blur} onKeyDown={onKeyDown} />
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
        <input id={id} type="date" disabled={disabled} value={value} onChange={(e) => onChange(e.target.value)} />
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
export function StCheckbox({ checked, onChange, label, disabled, indeterminate }: { checked: boolean; onChange?: (v: boolean) => void; label?: ReactNode; disabled?: boolean; indeterminate?: boolean }) {
  const box = (
    <span className="st-checkbox" data-checked={checked || indeterminate || undefined} aria-hidden={!onChange || undefined}>
      {checked && <IconCheck size={14} strokeWidth={3} />}
      {!checked && indeterminate && <span className="st-checkbox__dash" />}
    </span>
  );
  if (!onChange) return box;
  return (
    <label className="st-check" data-disabled={disabled || undefined}>
      <input type="checkbox" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
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
export function StSegmented<T extends string>({ label, value, options, onChange, size = "medium" }: { label?: ReactNode; value: T; options: StOption<T>[]; onChange: (v: T) => void; size?: "small" | "medium" }) {
  const id = useId();
  return (
    <FieldShell label={label} id={id}>
      <div id={id} className={`st-segmented st-segmented--${size}`} role="radiogroup">
        {options.map((o) => (
          <button key={o.value} type="button" role="radio" aria-checked={o.value === value} disabled={o.disabled} onClick={() => onChange(o.value)}>
            {o.label}
          </button>
        ))}
      </div>
    </FieldShell>
  );
}
