// The preview's pages. Go's report is one flowing HTML document that the
// WebView breaks into sheets only when it prints; the preview lays the same
// document out on sheets of the report's paper itself, so what is on which
// page shows before printing. The breaks are close to the printed ones, not
// identical: the WebView's own print layout has the last word.
//
// What the report tells the preview, as body attributes (internal/report
// render.go): data-paper "宽x高" and data-margin "上,右,下,左" in mm (the
// paper with its orientation applied); data-meet / data-title, the running
// head's texts from the second page on (none without data-meet), replaced by
// data-head-left / data-head-right; data-pagenum, the page number's format
// (n_of_m, dash, none), replaced by data-foot, a text with {page} and {pages}.

/** A sheet: the paper's size and margins, in mm. */
export interface Paper {
  width: number;
  height: number;
  /** top, right, bottom, left */
  margin: [number, number, number, number];
}

/** A4 with 15mm margins: the paper of a report that does not say. */
export const a4: Paper = { width: 210, height: 297, margin: [15, 15, 15, 15] };

const numbers = (text: string | undefined, count: number): number[] | null => {
  const parts = (text ?? "").split(/[x,]/).map((p) => Number(p));
  return parts.length === count && parts.every((n) => Number.isFinite(n) && n >= 0) ? parts : null;
};

/** The paper a report's body attributes name; what is missing or unreadable is A4's. */
export function readPaper(data: { paper?: string; margin?: string }): Paper {
  const size = numbers(data.paper, 2);
  const margin = numbers(data.margin, 4);
  return {
    width: size && size[0]! > 0 && size[1]! > 0 ? size[0]! : a4.width,
    height: size && size[0]! > 0 && size[1]! > 0 ? size[1]! : a4.height,
    margin: margin ? [margin[0]!, margin[1]!, margin[2]!, margin[3]!] : a4.margin,
  };
}

/** The rule that makes each printed page one sheet: its margins are inside the sheet. */
export const sheetRule = (paper: Paper) => `@page { size: ${paper.width}mm ${paper.height}mm; margin: 0; }`;

/** The text under page n of total: the report's own (with {page} and {pages}) or its format's. */
export function pageLabel(n: number, total: number, format?: string, foot?: string): string {
  if (foot) return foot.replaceAll("{pages}", String(total)).replaceAll("{page}", String(n));
  switch (format) {
    case "none": return "";
    case "dash": return `- ${n} -`;
    default: return `第 ${n} 页 / 共 ${total} 页`;
  }
}

/** The running head's left and right text; null when the report has none. */
export function headTexts(data: { meet?: string; title?: string; headLeft?: string; headRight?: string }): [string, string] | null {
  if (data.meet === undefined) return null;
  return [data.headLeft ?? data.meet, data.headRight ?? data.title ?? ""];
}

/** Zoom choices: fit the pane's width, or a fixed scale. */
export type Zoom = "fit" | "0.5" | "0.75" | "1" | "1.5";

// The space around a sheet; .pv-view and .pv-pages in internal/report/base.css use the same.
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
const splittable = new Set(["SECTION", "DIV", "TABLE", "TBODY", "MAIN", "ARTICLE", "UL", "OL", "BLOCKQUOTE", "FIGURE"]);
// Kept on one page while they fit one (break-inside: avoid in the report's CSS).
const keep = ".heat, .sheet, .line, .unit";
// Blocks that can carry a forced break of their own.
const blocks = "section, div, article, main, figure, ul, ol, blockquote, table, tbody, h1, h2, h3, h4, p, hr";
// Taller than a sheet, these are cut rather than split: a row, a cell or
// a table head split across sheets would no longer line up.
const unsplit = "tr, td, th, thead, li";
// What introduces the content after it: never the reason to start a new
// sheet, and taken along instead of being left at the bottom of one.
const heading = new Set(["H1", "H2", "H3", "H4", "P", "THEAD"]);

/**
 * Whether an element introduces what follows it (heading above). A report's
 * paragraphs are such lines (a scope, a note above a table); a paragraph of
 * a cover or a front page is content, and so is one that holds a picture.
 * Counted as a heading, content would never start a new sheet: the next
 * block that overflows would be cut off instead.
 */
export function isHeading(el: Element | null): boolean {
  if (!el || !heading.has(el.tagName)) return false;
  if (el.tagName !== "P") return true;
  return !el.closest(".front, .cover") && !el.querySelector("img, figure, table");
}

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
 * lines and page numbers included, on every WebView. The whole document is
 * serialized, so its head, the Content-Security-Policy meta included, goes
 * along.
 */
export function printable(doc: Document): string {
  const root = doc.documentElement.cloneNode(true) as HTMLElement;
  root.removeAttribute("style");
  for (const el of root.querySelectorAll(".pv-view, .pv-pages")) el.removeAttribute("style");
  return `<!DOCTYPE html>\n${root.outerHTML}`;
}

/**
 * Moves a report document's content onto sheets of the report's paper (A4
 * when it does not say) and returns how many there are. A container that does not fit continues on the next sheet,
 * tables with their header repeated; a heading is not left alone at the
 * bottom of a sheet. Forced breaks are read from the computed break-before.
 */
export async function paginate(doc: Document, signal?: AbortSignal): Promise<number> {
  if (signal?.aborted) return 0;
  const nodes = [...doc.body.childNodes];
  // Every sheet has its page number; books (body's data-meet) also have a
  // running head after the first sheet, whose title block says the same.
  const data = doc.body.dataset;
  const { pagenum, foot: footText } = data;
  const headText = headTexts(data);
  const paper = readPaper(data);
  // The sheets' look (.pv-*) is in internal/report/base.css; the paper's
  // size and margins are its variables, set from the same Params as @page.
  doc.documentElement.classList.add("pv");
  // Printed, each sheet is a page with its margins inside it. Only here:
  // Go's own document keeps @page's margins for printing it as it is.
  const sheets = doc.createElement("style");
  sheets.textContent = sheetRule(paper);
  doc.head.append(sheets);
  const view = doc.createElement("div");
  view.className = "pv-view";
  const pages = doc.createElement("div");
  pages.className = "pv-pages";
  view.append(pages);

  let sheet!: HTMLElement;
  let placed = 0; // nodes on the current sheet, headings aside
  let used = 0; // nodes on the current sheet, headings included
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
  const done = (node: Node) => { used++; if (!isHeading(element(node))) placed++; };
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
  // carry: a container left with nothing but headings takes them along. A
  // forced break does not: what is above it stays above it.
  const open = (carry = true) => {
    const before = [...stack];
    const previous = current;
    const page = doc.createElement("div");
    current = page;
    page.className = "pv-page";
    sheet = doc.createElement("div");
    sheet.className = "pv-body";
    page.append(sheet);
    if (headText && count > 0) {
      const running = doc.createElement("div");
      running.className = "pv-head";
      running.append(doc.createElement("span"), doc.createElement("span"));
      running.children[0]!.textContent = headText[0];
      running.children[1]!.textContent = headText[1];
      page.append(running);
    }
    const foot = doc.createElement("div");
    foot.className = "pv-foot";
    page.append(foot);
    pages.append(page);
    count++;
    placed = 0;
    used = 0;
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
    for (let i = before.length - 1; carry && i >= 0; i--) {
      const old = before[i]!;
      if (![...old.children].every((c) => isHeading(c))) break;
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
    // Anything on the sheet, a heading or a paragraph too, is above a
    // forced break: a front page of headings and paragraphs breaks where
    // its .page-break says.
    if (pageBreak && used > 0) {
      open(false);
    }
    const atomic = !el || !splittable.has(el.tagName) || el.children.length === 0;
    const box = boxes.get(node);
    const oversized = !atomic && box && box.bottom - box.top > sheet.clientHeight + 1;
    const inside = !atomic && breaksInside.has(el!);
    if (oversized) {
      if (placed > 0 && el!.matches(keep)) open();
    } else {
      top().append(node);
      if (!inside && !over()) { done(node); yield; return; }
      if (!inside && placed > 0 && (atomic || el!.matches(keep))) {
        node.parentNode?.removeChild(node);
        open();
        top().append(node);
        if (!over()) { done(node); yield; return; }
      }
      // Taller than a sheet on its own. A block of text and pictures (a
      // paragraph with an image, say) is split between its children, so
      // what follows the picture is not cut off with it; a table row or a
      // lone piece of text is cut at the sheet's edge.
      if (atomic && (!el || el.childNodes.length < 2 || el.matches(unsplit))) { done(node); yield; return; }
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

  // Containers with a forced break inside (a front page's .page-break, or
  // custom CSS) are laid out child by child even when they would fit
  // whole: placed whole, the break inside them would be skipped. Read
  // while the document is still attached, so the report's CSS applies.
  const breaksInside = new WeakSet<Element>();
  for (const node of nodes) forced(node);
  for (const el of doc.body.querySelectorAll(blocks)) {
    if (!forced(el)) continue;
    for (let up = el.parentElement; up && up !== doc.body && !breaksInside.has(up); up = up.parentElement) breaksInside.add(up);
  }
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
    foot.textContent = pageLabel(i + 1, count, pagenum, footText);
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
