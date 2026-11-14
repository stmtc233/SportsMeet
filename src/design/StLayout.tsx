import { useRef, type ReactNode } from "react";
import { useScrollMemory } from "../lib/viewState";
import { useSlideThumb } from "./motion";

export type Tone = "neutral" | "info" | "success" | "warning" | "danger";

/**
 * Page skeleton: a toolbar on top, the content scrolling under it. With a
 * scrollKey the body comes back at the position it was left at.
 */
export function StPage({ toolbar, children, flush, scrollKey }: { toolbar?: ReactNode; children: ReactNode; flush?: boolean; scrollKey?: string }) {
  const body = useRef<HTMLDivElement>(null);
  useScrollMemory(body, flush ? undefined : scrollKey);
  return (
    <div className="st-page">
      {toolbar}
      <div ref={body} className={flush ? "st-page__body st-page__body--flush" : "st-page__body"}>{children}</div>
    </div>
  );
}

/** Top toolbar: title on the left, actions right-aligned so they never shift. */
export function StToolbar({ title, subtitle, leading, children }: { title: ReactNode; subtitle?: ReactNode; leading?: ReactNode; children?: ReactNode }) {
  return (
    <header className="st-toolbar">
      {leading}
      <div className="st-toolbar__titles">
        <h1 className="st-toolbar__title">{title}</h1>
        {subtitle && <span className="st-toolbar__subtitle">{subtitle}</span>}
      </div>
      <div className="st-toolbar__actions">{children}</div>
    </header>
  );
}

export function StCard({ children, padded = true }: { children: ReactNode; padded?: boolean }) {
  return <section className={padded ? "st-card st-card--padded" : "st-card"}>{children}</section>;
}

/** A card with a title row and optional actions. */
export function StSection({ title, actions, children, padded = true }: { title: ReactNode; actions?: ReactNode; children: ReactNode; padded?: boolean }) {
  return (
    <section className="st-card">
      <div className="st-section__head">
        <h2 className="st-section__title">{title}</h2>
        <div className="st-section__actions">{actions}</div>
      </div>
      <div className={padded ? "st-section__body st-section__body--padded" : "st-section__body"}>{children}</div>
    </section>
  );
}

export function StNotice({ tone = "info", children }: { tone?: Tone; children: ReactNode }) {
  return (
    <div className={`st-notice st-tone-${tone}`} role={tone === "danger" ? "alert" : "status"}>
      {children}
    </div>
  );
}

/** A thin bar for how much of something is done; the label names it for screen readers. */
export function StProgress({ value, max, tone = "info", label }: { value: number; max: number; tone?: Tone; label: string }) {
  const ratio = max > 0 ? Math.min(1, Math.max(0, value / max)) : 0;
  return (
    <div className={`st-progress st-tone-${tone}`} role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={max} aria-valuenow={value}>
      <div className="st-progress__bar" style={{ transform: `scaleX(${ratio})` }} />
    </div>
  );
}

export function StTag({ tone = "neutral", children }: { tone?: Tone; children: ReactNode }) {
  return <span className={`st-tag st-tone-${tone}`}>{children}</span>;
}

export function StDivider() {
  return <hr className="st-divider" />;
}

/** What a list shows when it has nothing. */
export function StEmpty({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="st-empty">
      <span>{children}</span>
      {action}
    </div>
  );
}

/** A row of controls; bottoms align, since labels sit above the fields. */
export function StRow({ children, wrap = true }: { children: ReactNode; wrap?: boolean }) {
  return <div className={wrap ? "st-row st-row--wrap" : "st-row"}>{children}</div>;
}

/** A vertical stack with the standard gap. */
export function StStack({ children, gap = 12 }: { children: ReactNode; gap?: 4 | 8 | 12 | 16 | 24 }) {
  return <div className={`st-stack st-stack--${gap}`}>{children}</div>;
}

/** Muted secondary text. */
export function StMuted({ children }: { children: ReactNode }) {
  return <span className="st-muted">{children}</span>;
}

/** A settings-style row: the whole row is the click target. `indent` sets it one level in, under the row above (a tree). */
export function StListItem({ title, subtitle, leading, trailing, onClick, selected, disabled, indent }: {
  title: ReactNode;
  subtitle?: ReactNode;
  leading?: ReactNode;
  trailing?: ReactNode;
  onClick?: () => void;
  selected?: boolean;
  disabled?: boolean;
  indent?: boolean;
}) {
  const body = (
    <>
      {leading && <span className="st-list-item__leading">{leading}</span>}
      <span className="st-list-item__text">
        <span className="st-list-item__title">{title}</span>
        {subtitle && <span className="st-list-item__subtitle">{subtitle}</span>}
      </span>
      {trailing && <span className="st-list-item__trailing">{trailing}</span>}
    </>
  );
  if (!onClick) return <div className="st-list-item" data-indent={indent || undefined}>{body}</div>;
  // A div, not a button: the trailing slot may hold buttons of its own.
  return (
    <div role="button" tabIndex={disabled ? -1 : 0} className="st-list-item st-list-item--action" aria-current={selected || undefined} aria-disabled={disabled || undefined} data-indent={indent || undefined}
      onClick={disabled ? undefined : onClick}
      onKeyDown={(e) => {
        if (!disabled && e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onClick();
        }
      }}>
      {body}
    </div>
  );
}

/** A label over a stretch of StNavGroup items; the selected background slides across it. */
export function StNavHeading({ children }: { children: ReactNode }) {
  return <div className="st-nav-heading">{children}</div>;
}

/** A column of StListItems for navigation; the selected item's background slides to the next one. */
export function StNavGroup({ children }: { children: ReactNode }) {
  const thumb = useSlideThumb<HTMLDivElement>("y", "aria-current");
  return (
    <div ref={thumb.ref} className="st-nav-group" data-thumb={thumb.style ? true : undefined} data-animate={thumb.animate || undefined}>
      {thumb.style && <span className="st-nav-group__thumb" style={thumb.style} aria-hidden />}
      {children}
    </div>
  );
}
