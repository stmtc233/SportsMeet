import { useCallback, useEffect, useRef } from "react";
import { paginate, scalePages, type Zoom } from "./paginate";

/**
 * A report on A4 sheets. The frame runs no scripts (the sandbox only lets
 * this page reach its document to lay it out). Each report gets a frame of
 * its own: WKWebView drops the second of two quick changes to one frame's
 * srcdoc and stays blank. While the next report loads, and when there is
 * none, the one shown stays.
 */
export function ReportPreview({ html, zoom, onPages }: { html: string | undefined; zoom: Zoom; onPages: (n: number) => void }) {
  const box = useRef<HTMLDivElement>(null);
  const frame = useRef<HTMLIFrameElement>(null);
  const shown = useRef("");
  if (html) shown.current = html;
  const scale = useCallback(() => {
    const doc = frame.current?.contentDocument;
    if (!doc?.documentElement) return; // still loading
    scalePages(doc, zoom);
    // Around the sheets the frame shows the pane's colour. It is painted
    // rather than left transparent: a frame whose colour scheme differs
    // from the app's (a light report in the dark theme) is opaque white.
    if (box.current) doc.documentElement.style.background = getComputedStyle(box.current).backgroundColor;
  }, [zoom]);
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
  const laidOut = () => {
    const doc = frame.current?.contentDocument;
    if (!doc?.body || doc.querySelector(".pv-view")) return;
    onPages(paginate(doc));
    scale();
  };
  return (
    <div ref={box} className="report-frame">
      {shown.current && <iframe key={shown.current} ref={frame} title="预览" sandbox="allow-same-origin" srcDoc={shown.current} onLoad={laidOut} />}
    </div>
  );
}
