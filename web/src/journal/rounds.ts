// The round as the journal and the result card name it (ADR 0005; the glossary's Round).
import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import { formatTime } from "@/lib/format";

/** "Round 3" or "Out of combat" (round 0). */
export function roundLabel(t: TFunction, round: number): string {
  return round > 0 ? t("journal.round", { round }) : t("journal.outOfCombat");
}

/**
 * The journal's narrow round marker (ADR 0027 step 9.3): "T3" on a round's first row (null out of
 * combat, which shows an icon), and the full label for its tooltip and `aria-label`, "Round 3 ·
 * current" on the current round.
 */
export function roundMarker(
  t: TFunction,
  round: number,
  current: boolean,
): { short: string | null; full: string } {
  const label = roundLabel(t, round);
  return {
    short: round > 0 ? t("journal.roundShort", { round }) : null,
    full: current ? `${label} · ${t("log.current")}` : label,
  };
}

/** When an entry was logged: "T3 · 21:31", or the time alone out of combat. */
export function entryWhen(t: TFunction, e: LogEntry, lang: string): string {
  return t(e.round > 0 ? "journal.when" : "journal.whenOutOfCombat", {
    round: e.round,
    time: formatTime(e.createdAt, lang),
  });
}
