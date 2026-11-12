import type { ReactNode } from "react";

export type Tone = "neutral" | "info" | "success" | "warning" | "danger";

/** Page skeleton: a toolbar on top, the content scrolling under it. */
export function StPage({ toolbar, children, flush }: { toolbar?: ReactNode; children: ReactNode; flush?: boolean }) {
  return (
    <div className="st-page">
      {toolbar}
      <div className={flush ? "st-page__body st-page__body--flush" : "st-page__body"}>{children}</div>
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

/** A settings-style row: the whole row is the click target. */
export function StListItem({ title, subtitle, leading, trailing, onClick, selected, disabled }: {
  title: ReactNode;
  subtitle?: ReactNode;
  leading?: ReactNode;
  trailing?: ReactNode;
  onClick?: () => void;
  selected?: boolean;
  disabled?: boolean;
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
  if (!onClick) return <div className="st-list-item">{body}</div>;
  // A div, not a button: the trailing slot may hold buttons of its own.
  return (
    <div role="button" tabIndex={disabled ? -1 : 0} className="st-list-item st-list-item--action" aria-current={selected || undefined} aria-disabled={disabled || undefined}
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
