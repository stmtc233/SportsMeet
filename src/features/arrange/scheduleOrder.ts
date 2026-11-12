// The multi-round schedule dialog's order of rounds: pure, so it is tested
// without the page.

/** Picks an item (appended last) or drops it. */
export function toggleItem(order: string[], id: string): string[] {
  return order.includes(id) ? order.filter((x) => x !== id) : [...order, id];
}

/** Moves an item one place up (-1) or down (1); at either end it stays. */
export function moveItem(order: string[], id: string, delta: -1 | 1): string[] {
  const i = order.indexOf(id);
  const j = i + delta;
  if (i < 0 || j < 0 || j >= order.length) return order;
  const next = [...order];
  [next[i], next[j]] = [next[j]!, next[i]!];
  return next;
}
