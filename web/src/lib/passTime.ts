// Display only: the server converts hours to rounds and is the authority (buffsvc.RoundsForHours).
import type { TFunction } from "i18next";
import { parseDecimal } from "./decimalInput";

/** Rounds in one game hour (1 round = 1 minute), for the preview and the badge's hours. */
const ROUNDS_PER_HOUR = 60;

/**
 * The "= 90 rounds" preview of typed hours ("1.5" or "1,5"), rounded down like the server;
 * null when the text is not a number. 0 is returned as is: the server refuses it with a message.
 */
export function previewRounds(text: string): number | null {
  const hours = parseDecimal(text);
  if (Number.isNaN(hours)) return null;
  return Math.floor(hours * ROUNDS_PER_HOUR + 1e-9);
}

/**
 * A buff's rounds left: "59 rd" under an hour, "150 rd · 2 h 30" (or "120 rd · 2 h") from 60
 * rounds up.
 */
export function formatRoundsLeft(t: TFunction, rounds: number): string {
  const short = t("buffs.rounds", { count: rounds });
  if (rounds < ROUNDS_PER_HOUR) return short;
  const h = Math.floor(rounds / ROUNDS_PER_HOUR);
  const m = rounds % ROUNDS_PER_HOUR;
  const time =
    m === 0 ? t("buffs.hoursOnly", { h }) : t("buffs.hoursMinutes", { h, mm: String(m).padStart(2, "0") });
  return t("buffs.roundsWithTime", { rounds: short, time });
}
