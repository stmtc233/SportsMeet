// The preview's pages. Go's report is one flowing HTML document that the
// WebView breaks into sheets only when it prints; the preview lays the same
// document out on A4 sheets itself, so what is on which page shows before
// printing. The breaks are close to the printed ones, not identical: the
// WebView's own print layout has the last word.

/** Zoom choices: fit the pane's width, or a fixed scale. */
export type Zoom = "fit" | "0.5" | "0.75" | "1" | "1.5";

// The space around a sheet; .pv-view and .pv-pages in internal/report/style.css use the same.
const gutter = 16;

/** The scale that fits a page of pageWidth into a pane of paneWidth, gutters included. */
export function fitScale(paneWidth: number, pageWidth: number): number {
  if (pageWidth <= 0) return 1;
  return Math.max(0.2, (paneWidth - 2 * gutter) / pageWidth);
}

/** The scale a zoom choice gives. */
export const zoomScale = (zoom: Zoom, paneWidth: number, pageWidth: number) => (zoom === "fit" ? fitScale(paneWidth, pageWidth) : Number(zoom));

// Containers whose children may go on different pages; everything else
// (rows, headings, paragraphs) moves as a whole.
const splittable = new Set(["SECTION", "DIV", "TABLE", "TBODY", "MAIN", "ARTICLE"]);
// Kept on one page while they fit one (break-inside: avoid in the report's CSS).
const keep = ".heat, .sheet, .line";
// What introduces the content after it: never the reason to start a new
// sheet, and taken along instead of being left at the bottom of one.
const heading = new Set(["H1", "H2", "H3", "H4", "P", "THEAD"]);

/**
 * Moves a report document's content onto A4 sheets and returns how many
 * there are. A container that does not fit continues on the next sheet,
 * tables with their header repeated; a heading is not left alone at the
 * bottom of a sheet. Forced breaks are read from the computed break-before.
 */
export function paginate(doc: Document): number {
  const nodes = [...doc.body.childNodes];
  // The sheets' look (.pv-*) is in internal/report/style.css, beside @page.
  doc.documentElement.classList.add("pv");
  const view = doc.createElement("div");
  view.className = "pv-view";
  const pages = doc.createElement("div");
  pages.className = "pv-pages";
  view.append(pages);
  doc.body.replaceChildren(view);

  let sheet!: HTMLElement;
  let placed = 0; // nodes on the current sheet, headings aside
  let count = 0;
  const stack: Element[] = []; // open containers on the current sheet, outermost first
  const top = () => stack[stack.length - 1] ?? sheet;
  const over = () => sheet.scrollHeight > sheet.clientHeight + 1;

  const open = () => {
    const before = [...stack];
    const page = doc.createElement("div");
    page.className = "pv-page";
    sheet = doc.createElement("div");
    sheet.className = "pv-body";
    page.append(sheet);
    pages.append(page);
    count++;
    placed = 0;
    let parent: Element = sheet;
    for (let i = 0; i < before.length; i++) {
      const clone = before[i]!.cloneNode(false) as Element;
      const head = clone.tagName === "TABLE" ? before[i]!.querySelector(":scope > thead") : null;
      if (head) clone.append(head.cloneNode(true));
      parent.append(clone);
      stack[i] = clone;
      parent = clone;
    }
    // A container left with nothing but headings takes them along.
    for (let i = before.length - 1; i >= 0; i--) {
      const old = before[i]!;
      if (![...old.children].every((c) => heading.has(c.tagName))) break;
      const lead = [...old.children].filter((c) => c.tagName !== "THEAD");
      stack[i]!.prepend(...lead);
      old.remove();
    }
  };

  const place = (node: Node) => {
    if (node.nodeType === Node.TEXT_NODE && !node.textContent?.trim()) return;
    if (node.nodeType !== Node.ELEMENT_NODE && node.nodeType !== Node.TEXT_NODE) return;
    const el = node instanceof (doc.defaultView as typeof globalThis).Element ? (node as HTMLElement) : null;
    if (el && (el.tagName === "STYLE" || el.tagName === "SCRIPT")) return;
    top().append(node);
    if (el && placed > 0) {
      const cs = doc.defaultView!.getComputedStyle(el);
      if (cs.breakBefore === "page" || cs.getPropertyValue("page-break-before") === "always") {
        el.remove();
        open();
        top().append(el);
      }
    }
    const done = () => {
      if (!el || !heading.has(el.tagName)) placed++;
    };
    if (!over()) return done();
    const atomic = !el || !splittable.has(el.tagName) || el.children.length === 0;
    if (placed > 0 && (atomic || el!.matches(keep))) {
      node.parentNode?.removeChild(node);
      open();
      top().append(node);
      if (!over()) return done();
    }
    if (atomic) return done(); // taller than a sheet: cut off
    const kids = [...el!.childNodes];
    const shell = el!.cloneNode(false) as Element;
    el!.replaceWith(shell);
    stack.push(shell);
    for (const k of kids) place(k);
    stack.pop();
  };

  open();
  for (const n of nodes) place(n);
  return count;
}

/** Scales the sheets and sizes their box so the pane scrolls by what is shown. */
export function scalePages(doc: Document, zoom: Zoom): void {
  const view = doc.querySelector<HTMLElement>(".pv-view");
  const pages = doc.querySelector<HTMLElement>(".pv-pages");
  if (!view || !pages) return;
  const scale = zoomScale(zoom, doc.documentElement.clientWidth, pages.offsetWidth);
  pages.style.transform = `scale(${scale})`;
  view.style.width = `${pages.offsetWidth * scale}px`;
  view.style.height = `${pages.offsetHeight * scale}px`;
}
