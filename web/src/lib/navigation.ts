import { useSyncExternalStore } from "react";

/** True while `navigate` announces its own move: the app's navigation is never held. */
let announcing = false;

/**
 * Client navigation between the app and the creation route (ADR 0034 step 2), without a reload: the
 * path changes in the history and the root re-renders on it; Back and Forward do the same.
 */
export function navigate(path: string): void {
  if (location.pathname === path) return;
  history.pushState(null, "", path);
  announcing = true;
  try {
    dispatchEvent(new PopStateEvent("popstate"));
  } finally {
    announcing = false;
  }
}

let hold: { path: string; onHeld: () => void } | null = null;

/**
 * The browser's Back or Forward away from a held path: the path is put back before anything
 * re-renders on it, and the holder is told (it asks the player, then leaves through `navigate`).
 * Registered once, before any `usePath` subscriber, so it runs first and stops the event.
 */
function holdBack(e: PopStateEvent): void {
  if (announcing || !hold || location.pathname === hold.path) return;
  history.pushState(null, "", hold.path);
  e.stopImmediatePropagation();
  hold.onHeld();
}
globalThis.addEventListener?.("popstate", holdBack);

/**
 * Holds the current path against the browser's Back and Forward (a page with values entered, where
 * `beforeunload` does not fire because the document stays): `onHeld` runs instead of leaving.
 * Returns the release.
 */
export function holdPath(onHeld: () => void): () => void {
  const mine = { path: location.pathname, onHeld };
  hold = mine;
  return () => {
    if (hold === mine) hold = null;
  };
}

function subscribe(onChange: () => void): () => void {
  addEventListener("popstate", onChange);
  return () => removeEventListener("popstate", onChange);
}

/** The current path, re-rendering on `navigate` and on Back / Forward. */
export function usePath(): string {
  return useSyncExternalStore(subscribe, () => location.pathname);
}
