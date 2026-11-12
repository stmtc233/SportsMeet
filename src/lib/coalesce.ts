// Event-driven reloads are merged (docs/agents/frontend.md): the first call
// runs at once, later calls within the window collapse into one at its end.
export function coalesce(fn: () => Promise<void>, windowMs = 500): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending = false;
  const run = () => {
    fn().catch((err) => console.error(err));
  };
  return () => {
    if (timer !== undefined) {
      pending = true;
      return;
    }
    run();
    timer = setTimeout(function tick() {
      if (pending) {
        pending = false;
        run();
        timer = setTimeout(tick, windowMs);
      } else {
        timer = undefined;
      }
    }, windowMs);
  };
}
