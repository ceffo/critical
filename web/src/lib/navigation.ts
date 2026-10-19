import { useSyncExternalStore } from "react";

/**
 * Client navigation between the app and the creation route (ADR 0034 step 2), without a reload: the
 * path changes in the history and the root re-renders on it; Back and Forward do the same.
 */
export function navigate(path: string): void {
  if (location.pathname === path) return;
  history.pushState(null, "", path);
  dispatchEvent(new PopStateEvent("popstate"));
}

function subscribe(onChange: () => void): () => void {
  addEventListener("popstate", onChange);
  return () => removeEventListener("popstate", onChange);
}

/** The current path, re-rendering on `navigate` and on Back / Forward. */
export function usePath(): string {
  return useSyncExternalStore(subscribe, () => location.pathname);
}
