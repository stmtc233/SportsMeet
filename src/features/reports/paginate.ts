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
const keep = ".heat, .sheet, .line, .unit";
// What introduces the content after it: never the reason to start a new
// sheet, and taken along instead of being left at the bottom of one.
const heading = new Set(["H1", "H2", "H3", "H4", "P", "THEAD"]);

/** The headings a container gives its continuation: its title and section, not the group's. */
const titled = new Set(["H2", "H3", "H4"]);

/** A heading's own words, without the details in its <small>. */
const ownText = (el: Element) => [...el.childNodes].filter((n) => n.nodeType === 3).map((n) => n.textContent).join("").trim();

/**
 * What a new sheet continues, from the containers split across the break,
 * outermost first: each one's last heading of each level before where it
 * breaks (男子100m 初一组 · 决赛). The group title of a 成绩册 is left out:
 * the event names say the group. "" when nothing is titled.
 */
export function continuedTitle(open: readonly Element[]): string {
  const parts: string[] = [];
  open.forEach((container, i) => {
    const last = new Map<string, string>();
    for (const child of container.children) {
      if (child === open[i + 1]) break;
      if (titled.has(child.tagName) && !child.classList.contains("report-group__title")) last.set(child.tagName, ownText(child));
    }
    for (const tag of titled) {
      const text = last.get(tag);
      if (text) parts.push(text);
    }
  });
  return parts.join(" · ");
}

/**
 * The laid-out sheets as a document of their own, for the print window: at
 * scale 1, without the pane's colour. Printing them rather than Go's
 * document keeps the printed pages what the preview shows, continuation
 * lines and page numbers included, on every WebView.
 */
export function printable(doc: Document): string {
  const root = doc.documentElement.cloneNode(true) as HTMLElement;
  root.removeAttribute("style");
  for (const el of root.querySelectorAll(".pv-view, .pv-pages")) el.removeAttribute("style");
  return `<!DOCTYPE html>\n${root.outerHTML}`;
}

/**
 * Moves a report document's content onto A4 sheets and returns how many
 * there are. A container that does not fit continues on the next sheet,
 * tables with their header repeated; a heading is not left alone at the
 * bottom of a sheet. Forced breaks are read from the computed break-before.
 */
export async function paginate(doc: Document, signal?: AbortSignal): Promise<number> {
  if (signal?.aborted) return 0;
  const nodes = [...doc.body.childNodes];
  // Every sheet has its page number; books (body's data-meet) also have a
  // running head after the first sheet, whose title block says the same.
  const { meet, title } = doc.body.dataset;
  // The sheets' look (.pv-*) is in internal/report/style.css, beside @page.
  doc.documentElement.classList.add("pv");
  // Printed, each sheet is a page with its margins inside it. Only here:
  // Go's own document keeps @page's margins for printing it as it is.
  const sheets = doc.createElement("style");
  sheets.textContent = "@page { size: A4; margin: 0; }";
  doc.head.append(sheets);
  const view = doc.createElement("div");
  view.className = "pv-view";
  const pages = doc.createElement("div");
  pages.className = "pv-pages";
  view.append(pages);

  let sheet!: HTMLElement;
  let placed = 0; // nodes on the current sheet, headings aside
  let count = 0;
  const stack: Element[] = []; // open containers on the current sheet, outermost first
  const top = () => stack[stack.length - 1] ?? sheet;
  const over = () => sheet.scrollHeight > sheet.clientHeight + 1;
  const breaks = new WeakMap<Element, boolean>();
  const boxes = new WeakMap<Node, { top: number; bottom: number }>();
  const element = (node: Node) => node.nodeType === Node.ELEMENT_NODE ? node as HTMLElement : null;
  const layoutScale = () => pages.getBoundingClientRect().width / pages.offsetWidth || 1;
  const bounds = (el: Element, scale: number) => {
    const box = el.getBoundingClientRect();
    return { top: box.top / scale, bottom: box.bottom / scale };
  };
  const forced = (node: Node) => {
    const el = element(node);
    if (!el) return false;
    if (!breaks.has(el)) {
      const cs = doc.defaultView!.getComputedStyle(el);
      breaks.set(el, cs.breakBefore === "page" || cs.getPropertyValue("page-break-before") === "always");
    }
    return breaks.get(el)!;
  };
  const done = (node: Node) => { if (!heading.has(element(node)?.tagName ?? "")) placed++; };
  const measure = (root: Element) => {
    const scale = layoutScale();
    // One connected read pass. Oversized nested containers can then be split
    // directly, without laying out the same complete subtree at every level.
    for (const el of [root, ...root.querySelectorAll("section, div, table, tbody, main, article, tr, h1, h2, h3, h4, p, thead")]) {
      if (!boxes.has(el)) boxes.set(el, bounds(el, scale));
      forced(el);
    }
  };

  let current: HTMLElement | null = null;
  const open = () => {
    const before = [...stack];
    const previous = current;
    const page = doc.createElement("div");
    current = page;
    page.className = "pv-page";
    sheet = doc.createElement("div");
    sheet.className = "pv-body";
    page.append(sheet);
    if (meet !== undefined && count > 0) {
      const head = doc.createElement("div");
      head.className = "pv-head";
      head.append(doc.createElement("span"), doc.createElement("span"));
      head.children[0]!.textContent = meet;
      head.children[1]!.textContent = title ?? "";
      page.append(head);
    }
    const foot = doc.createElement("div");
    foot.className = "pv-foot";
    page.append(foot);
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
    // A table cut between its rows says so at both ends of the break:
    // 续下页 right under its last row, and what it continues above the rest.
    // Anything else starts the new sheet with its own heading (a round, a
    // heat, a unit), which says what it is already.
    const kept = before.filter((el) => el.isConnected); // those whose headings came along start afresh
    const table = kept.find((el) => el.tagName === "TABLE");
    const text = table && previous ? continuedTitle(kept) : "";
    if (table && previous && text) {
      const scale = layoutScale();
      const more = doc.createElement("div");
      more.className = "pv-more";
      more.textContent = "续下页";
      more.style.top = `${(table.getBoundingClientRect().bottom - previous.getBoundingClientRect().top) / scale}px`;
      previous.append(more);
      const cont = doc.createElement("p");
      cont.className = "pv-cont";
      cont.textContent = `${text}（续）`;
      sheet.prepend(cont);
    }
  };

  // Descendants are measured together while their original container is still
  // attached. Use those positions to fill a page, then verify the whole batch.
  // The actual layout remains authoritative for auto-width tables and margins.
  function* batch(nodes: Node[]): Generator<void> {
    let i = 0;
    while (i < nodes.length) {
      const first = nodes[i++]!;
      yield* place(first);
      const measured = boxes.get(first);
      if (!measured || !first.isConnected) continue; // an oversized block split
      const scale = layoutScale();
      const anchor = bounds(element(first)!, scale).top;
      const bottom = bounds(sheet, scale).top + sheet.clientHeight + 1;
      let end = i;
      while (end < nodes.length) {
        const box = boxes.get(nodes[end]!);
        if (!box || anchor + box.bottom - measured.top > bottom) break;
        end++;
      }
      if (end === i) continue;
      const fragment = doc.createDocumentFragment();
      for (let k = i; k < end; k++) fragment.append(nodes[k]!);
      top().append(fragment);
      while (end > i && over()) { end--; nodes[end]!.parentNode?.removeChild(nodes[end]!); }
      for (let k = i; k < end; k++) done(nodes[k]!);
      i = end;
      yield;
    }
  }
  function* children(nodes: Node[]): Generator<void> {
    let pending: Node[] = [];
    for (const node of nodes) {
      if (node.nodeType === Node.TEXT_NODE && !node.textContent?.trim()) continue;
      if (node.nodeType !== Node.ELEMENT_NODE && node.nodeType !== Node.TEXT_NODE) continue;
      if (element(node)?.matches("style, script")) continue;
      const el = element(node);
      // Only batch sibling rows/atomic blocks. Appending many whole event
      // trees only to split them repeatedly makes layout more expensive.
      const groupable = el && (el.tagName === "TR" || el.matches(keep));
      if (forced(node) || !groupable) {
        if (pending.length) { yield* batch(pending); pending = []; }
        yield* place(node);
      } else {
        pending.push(node);
      }
    }
    if (pending.length) yield* batch(pending);
  }
  function* place(node: Node): Generator<void> {
    if (node.nodeType === Node.TEXT_NODE && !node.textContent?.trim()) return;
    if (node.nodeType !== Node.ELEMENT_NODE && node.nodeType !== Node.TEXT_NODE) return;
    const el = element(node);
    if (el && (el.tagName === "STYLE" || el.tagName === "SCRIPT")) return;
    const pageBreak = forced(node);
    if (pageBreak && placed > 0) {
      open();
    }
    const atomic = !el || !splittable.has(el.tagName) || el.children.length === 0;
    const box = boxes.get(node);
    const oversized = !atomic && box && box.bottom - box.top > sheet.clientHeight + 1;
    if (oversized) {
      if (placed > 0 && el!.matches(keep)) open();
    } else {
      top().append(node);
      if (!over()) { done(node); yield; return; }
      if (placed > 0 && (atomic || el!.matches(keep))) {
        node.parentNode?.removeChild(node);
        open();
        top().append(node);
        if (!over()) { done(node); yield; return; }
      }
      if (atomic) { done(node); yield; return; } // taller than a sheet: cut off
      measure(el!);
    }
    const kids = [...el!.childNodes];
    // Read while the original descendants are still connected; computed
    // styles on detached nodes do not include the report's CSS rules.
    const scale = layoutScale();
    for (const kid of kids) {
      if (kid.isConnected) {
        forced(kid);
        const el = element(kid);
        if (el && !boxes.has(kid)) boxes.set(kid, bounds(el, scale));
      }
    }
    const shell = el!.cloneNode(false) as Element;
    if (oversized) { el!.remove(); top().append(shell); } else { el!.replaceWith(shell); }
    stack.push(shell);
    yield* children(kids);
    stack.pop();
  }

  for (const node of nodes) forced(node);
  doc.body.replaceChildren(view);
  open();
  function* roots() { for (const node of nodes) yield* place(node); }
  const work = roots();
  let deadline = performance.now() + 8;
  for (const _ of work) {
    if (signal?.aborted) return 0;
    if (performance.now() >= deadline) {
      // MessageChannel avoids the nested setTimeout minimum delay while
      // allowing input and rendering between chunks in Chrome and WebKit.
      await new Promise<void>((resolve) => {
        const channel = new MessageChannel();
        channel.port1.onmessage = () => { channel.port1.close(); channel.port2.close(); resolve(); };
        channel.port2.postMessage(null);
      });
      if (signal?.aborted) return 0;
      deadline = performance.now() + 8;
    }
  }
  pages.querySelectorAll(":scope > .pv-page > .pv-foot").forEach((foot, i) => {
    foot.textContent = `第 ${i + 1} 页 / 共 ${count} 页`;
  });
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
