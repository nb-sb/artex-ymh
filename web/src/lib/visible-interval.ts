/** Run `fn` on an interval, skipping ticks while the document is hidden.
 *  Fires once when the tab becomes visible again. */
export function startVisibleInterval(fn: () => void, ms: number): () => void {
  let timer: ReturnType<typeof setInterval> | undefined;
  const tick = () => {
    if (typeof document !== "undefined" && document.hidden) return;
    fn();
  };
  const onVisibility = () => {
    if (!document.hidden) fn();
  };
  fn();
  timer = setInterval(tick, ms);
  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", onVisibility);
  }
  return () => {
    if (timer) clearInterval(timer);
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", onVisibility);
    }
  };
}
