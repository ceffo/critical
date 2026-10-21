import { useEffect, useState } from "react";
import { fetchBuild } from "@/api/health";
import {
  nextStreak,
  parseRelease,
  POLL_INTERVAL_MS,
  updateAvailable,
} from "@/lib/updateCheck";

// The count of consecutive newer answers lives outside the components: each screen has its own
// badge, and moving between screens must neither reset it nor show the notice twice.
let streak = 0;
const listeners = new Set<() => void>();

function record(running: string, answer: string | null) {
  const before = updateAvailable(streak);
  streak = nextStreak(streak, running, answer);
  if (updateAvailable(streak) !== before) listeners.forEach((l) => l());
}

/**
 * True once two consecutive health answers report a newer version than `running`. The page is only
 * asked while the tab is visible: at once when it comes back to the front, then on a slow timer. A
 * hidden tab sends nothing, so an auto-stopping staging machine can still stop. Nothing is checked
 * for a running version that cannot be compared (a dev build, "unknown").
 */
export function useNewVersion(running: string | null): boolean {
  const [, bump] = useState(0);
  const shown = updateAvailable(streak);
  useEffect(() => {
    const l = () => bump((n) => n + 1);
    listeners.add(l);
    return () => void listeners.delete(l);
  }, []);

  useEffect(() => {
    if (!running || !parseRelease(running) || shown) return;
    let timer: ReturnType<typeof setInterval> | undefined;
    let live = true;
    let last = 0;
    const check = () => {
      // A tab switch fires both a visibility change and a focus: one answer, not two.
      if (Date.now() - last < 2000) return;
      last = Date.now();
      void fetchBuild().then((b) => {
        if (live) record(running, b?.version ?? null);
      });
    };
    const sync = () => {
      clearInterval(timer);
      timer = undefined;
      if (document.visibilityState !== "visible") return;
      check();
      timer = setInterval(check, POLL_INTERVAL_MS);
    };
    document.addEventListener("visibilitychange", sync);
    window.addEventListener("focus", sync);
    // Only the timer on mount: the page just loaded this version.
    if (document.visibilityState === "visible")
      timer = setInterval(check, POLL_INTERVAL_MS);
    return () => {
      live = false;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", sync);
      window.removeEventListener("focus", sync);
    };
  }, [running, shown]);

  return running !== null && shown;
}
