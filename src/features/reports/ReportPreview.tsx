import { useCallback, useEffect, useRef, useState, type MutableRefObject } from "react";
import { paginate, printable, scalePages, type Zoom } from "./paginate";

/** The sheets shown, ready for the print window; null while none is laid out. */
export type PrintPages = () => { title: string; html: string } | null;

interface Sheet {
  html: string;
  /** Laid out on sheets: shown. Until then it loads under the shown one. */
  ready: boolean;
  /** Stacking order, fixed when the frame is made: newer frames lie under older ones. */
  layer: number;
}

let layers = 0;

/**
 * A report on A4 sheets. The frame runs no scripts (the sandbox only lets
 * this page reach its document to lay it out). Each report gets a frame of
 * its own: WKWebView drops the second of two quick changes to one frame's
 * srcdoc and stays blank. The next report loads and is paginated in a
 * hidden frame under the one shown, which stays until then; when there is
 * none, the one shown stays too.
 *
 * A frame's style never changes once made, only its place in the stack:
 * WebKit can keep treating a frame made with pointer-events: none (or
 * hidden) as not scrollable after the style goes, so the wheel scrolls
 * the window instead. The first report is hidden by a cover instead.
 */
export function ReportPreview({ html, zoom, onPages, printRef }: {
  html: string | undefined;
  zoom: Zoom;
  /** The report now shown and its number of sheets. */
  onPages: (n: number, html: string) => void;
  printRef: MutableRefObject<PrintPages | null>;
}) {
  const box = useRef<HTMLDivElement>(null);
  const frames = useRef(new Map<string, HTMLIFrameElement>());
  const pagination = useRef<AbortController | null>(null);
  const [sheets, setSheets] = useState<Sheet[]>([]);
  const shown = useRef<string | null>(null);
  useEffect(() => () => pagination.current?.abort(), []);
  useEffect(() => {
    printRef.current = () => {
      const doc = shown.current === null ? null : frames.current.get(shown.current)?.contentDocument;
      return doc?.querySelector(".pv-view") ? { title: doc.title, html: printable(doc) } : null;
    };
    return () => { printRef.current = null; };
  }, [printRef]);
  useEffect(() => {
    if (!html) return;
    setSheets((old) => {
      if (old.at(-1)?.html === html) return old;
      // At most the one shown and the newest one loading.
      const shown = old.filter((s) => s.ready);
      if (shown.some((s) => s.html === html)) return shown;
      return [...shown, { html, ready: false, layer: --layers }];
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
        shown.current = key;
        onPages(pages, key);
        setSheets((old) => {
          const at = old.findIndex((s) => s.html === key);
          return at < 0 ? old : [{ ...old[at]!, ready: true }, ...old.slice(at + 1)];
        });
      }
    }
  };
  return (
    <div ref={box} className="report-frame">
      {sheets.map((s) => (
        <iframe key={s.html} title={s.ready ? "预览" : "下一份预览"} aria-hidden={!s.ready || undefined} style={{ zIndex: s.layer }}
          sandbox="allow-same-origin" srcDoc={s.html} onLoad={() => void laidOut(s.html)}
          ref={(el) => {
            if (el) frames.current.set(s.html, el);
            else frames.current.delete(s.html);
          }} />
      ))}
      {!sheets.some((s) => s.ready) && sheets.length > 0 && <div className="report-frame__cover" />}
    </div>
  );
}
