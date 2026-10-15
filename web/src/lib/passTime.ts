// Display only: the server converts hours to rounds and is the authority (buffsvc.RoundsForHours,
// GET /api/round/pass-time/preview).
import type { TFunction } from "i18next";

/** Turns (rounds) in one game hour (1 turn = 1 minute), for the pill's hours. */
const TURNS_PER_HOUR = 60;

/**
 * A buff's turns left: "59 turns" under an hour, "150 turns · 2 h 30" (or "120 turns · 2 h") from
 * 60 turns up.
 */
export function formatTurnsLeft(t: TFunction, turns: number): string {
  const short = t("effects.turns", { count: turns });
  if (turns < TURNS_PER_HOUR) return short;
  const h = Math.floor(turns / TURNS_PER_HOUR);
  const m = turns % TURNS_PER_HOUR;
  const time =
    m === 0 ? t("buffs.hoursOnly", { h }) : t("buffs.hoursMinutes", { h, mm: String(m).padStart(2, "0") });
  return t("buffs.roundsWithTime", { rounds: short, time });
}
