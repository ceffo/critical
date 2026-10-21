/**
 * When the app tells the user a newer version is deployed. The rule is pure so a test can drive it
 * with fake answers; the polling and the toast are in `components/shell`.
 */

/** Consecutive answers that must report a newer version before the notice shows: a rolling deploy briefly has two machines on two versions. */
export const NEWER_ANSWERS_NEEDED = 2;

/** How often a visible tab re-reads the health answer, besides the check when the tab regains focus. */
export const POLL_INTERVAL_MS = 10 * 60 * 1000;

/**
 * A released version, `X.Y.Z` as [major, minor, patch]. Anything else is not comparable and so never
 * triggers the notice: "unknown", and a dev build (`X.Y.Z+<sha>[.dirty]`, whose number is only the
 * last tag).
 */
export function parseRelease(version: string): [number, number, number] | null {
  const m = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(version.trim());
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

/** True when `answer` is a strictly newer release than `running` (an equal, older or unreadable one is not). */
export function isNewer(running: string, answer: string): boolean {
  const a = parseRelease(running);
  const b = parseRelease(answer);
  if (!a || !b) return false;
  for (let i = 0; i < 3; i++) {
    if (b[i]! !== a[i]!) return b[i]! > a[i]!;
  }
  return false;
}

/** The count of consecutive newer answers after one more `answer`: any answer that is not newer starts over. */
export function nextStreak(
  streak: number,
  running: string,
  answer: string | null,
): number {
  return answer !== null && isNewer(running, answer) ? streak + 1 : 0;
}

/** Whether the notice shows after that many consecutive newer answers. */
export function updateAvailable(streak: number): boolean {
  return streak >= NEWER_ANSWERS_NEEDED;
}
