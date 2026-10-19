// Display only: the server converts hours to rounds and is the authority (buffsvc.RoundsForHours,
// GET /api/round/pass-time/preview).
import type { TFunction } from "i18next";

/** Rounds in one game hour (1 round = 1 minute), for the pill's hours. */
const ROUNDS_PER_HOUR = 60;

/**
 * A buff's rounds left: "59 rounds" under an hour, "150 rounds · 2 h 30" (or "120 rounds · 2 h") from
 * 60 rounds up.
 */
export function formatRoundsLeft(t: TFunction, rounds: number): string {
  const short = t("effects.rounds", { count: rounds });
  if (rounds < ROUNDS_PER_HOUR) return short;
  const h = Math.floor(rounds / ROUNDS_PER_HOUR);
  const m = rounds % ROUNDS_PER_HOUR;
  const time =
    m === 0
      ? t("buffs.hoursOnly", { h })
      : t("buffs.hoursMinutes", { h, mm: String(m).padStart(2, "0") });
  return t("buffs.roundsWithTime", { rounds: short, time });
}
