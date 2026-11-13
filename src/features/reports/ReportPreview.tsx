import { useCallback, useEffect, useRef, useState } from "react";
import { paginate, scalePages, type Zoom } from "./paginate";

interface Sheet {
  html: string;
  /** Laid out on sheets: shown. Until then it loads under the shown one. */
  ready: boolean;
}

/**
 * A report on A4 sheets. The frame runs no scripts (the sandbox only lets
 * this page reach its document to lay it out). Each report gets a frame of
 * its own: WKWebView drops the second of two quick changes to one frame's
 * srcdoc and stays blank. The next report loads and is paginated in a
 * hidden frame under the one shown, which stays until then; when there is
 * none, the one shown stays too.
 */
export function ReportPreview({ html, zoom, onPages }: { html: string | undefined; zoom: Zoom; onPages: (n: number) => void }) {
  const box = useRef<HTMLDivElement>(null);
  const frames = useRef(new Map<string, HTMLIFrameElement>());
  const pagination = useRef<AbortController | null>(null);
  const [sheets, setSheets] = useState<Sheet[]>([]);
  useEffect(() => () => pagination.current?.abort(), []);
  useEffect(() => {
    if (!html) return;
    setSheets((old) => {
      if (old.at(-1)?.html === html) return old;
      // At most the one shown and the newest one loading.
      const shown = old.filter((s) => s.ready);
      if (shown.some((s) => s.html === html)) return shown;
      return [...shown, { html, ready: false }];
    });
  }, [html]);

  const scaleOne = useCallback((doc: Document | null | undefined) => {
    if (!doc?.documentElement) return; // still loading
    scalePages(doc, zoom);
    // Around the sheets the frame shows the pane's colour. It is painted
    // rather than left transparent: a frame whose colour scheme differs
    // from the app's (a light report in the dark theme) is opaque white.
    if (box.current) doc.documentElement.style.background = getComputedStyle(box.current).backgroundColor;
  }, [zoom]);
  const scale = useCallback(() => frames.current.forEach((f) => scaleOne(f.contentDocument)), [scaleOne]);
  useEffect(() => {
    scale();
    const el = box.current;
    if (!el) return;
    const watch = new ResizeObserver(scale);
    watch.observe(el);
    // The theme: the setting (data-theme on the root) or the system's.
    const theme = new MutationObserver(scale);
    theme.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    const system = matchMedia("(prefers-color-scheme: dark)");
    system.addEventListener("change", scale);
    return () => {
      watch.disconnect();
      theme.disconnect();
      system.removeEventListener("change", scale);
    };
  }, [scale]);

  const laidOut = async (key: string) => {
    const frame = frames.current.get(key);
    const doc = frame?.contentDocument;
    if (!doc?.body || doc.querySelector(".pv-view")) return;
    pagination.current?.abort();
    const run = new AbortController();
    pagination.current = run;
    let pages = 0;
    try {
      pages = await paginate(doc, run.signal);
    } finally {
      // A failed layout still replaces the old report rather than hiding the new one.
      if (!run.signal.aborted && frames.current.get(key)?.contentDocument === doc) {
        scaleOne(doc);
        onPages(pages);
        setSheets((old) => {
          const at = old.findIndex((s) => s.html === key);
          return at < 0 ? old : [{ html: key, ready: true }, ...old.slice(at + 1)];
        });
      }
    }
  };
  return (
    <div ref={box} className="report-frame">
      {sheets.map((s) => (
        <iframe key={s.html} title={s.ready ? "预览" : "下一份预览"} aria-hidden={!s.ready || undefined} data-pending={!s.ready || undefined}
          sandbox="allow-same-origin" srcDoc={s.html} onLoad={() => void laidOut(s.html)}
          ref={(el) => {
            if (el) frames.current.set(s.html, el);
            else frames.current.delete(s.html);
          }} />
      ))}
    </div>
  );
}
