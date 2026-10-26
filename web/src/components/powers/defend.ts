// What the defend form (ADR 0044 step 8) sends and the one thing it checks before the server does.
// The contests, the checks and the winners are the server's.
import type { TFunction } from "i18next";
import type { IncomingRoll } from "@/api/psionics";

export const CHECK_DIE = 20;

export interface DefendDraft {
  attack: string;
  rolls: { roll: string; success: boolean }[];
  /** Index of the roll whose check is kept, or null. */
  keep: number | null;
}

export const EMPTY_DEFEND: DefendDraft = {
  attack: "",
  rolls: [{ roll: "", success: false }],
  keep: null,
};

/** The request the draft makes, or null while the attack is unchosen or a roll is not 1 to 20. */
export function defendRequest(
  d: DefendDraft,
): { attack: string; rolls: IncomingRoll[]; keep?: number } | null {
  if (!d.attack) return null;
  const rolls: IncomingRoll[] = [];
  for (const r of d.rolls) {
    const t = r.roll.trim();
    if (!/^\d+$/.test(t)) return null;
    const n = Number.parseInt(t, 10);
    if (n < 1 || n > CHECK_DIE) return null;
    rolls.push({ roll: n, success: r.success });
  }
  const req: { attack: string; rolls: IncomingRoll[]; keep?: number } = {
    attack: d.attack,
    rolls,
  };
  if (d.keep !== null && d.keep < rolls.length) req.keep = d.keep;
  return req;
}

/** How a defence check was decided, as words: the die against the target, the outcome, the kept +1. */
export function checkText(
  c: { target: number; d20: number; outcome: string; kept?: boolean },
  fromKept: boolean | undefined,
  t: TFunction,
): string {
  const parts = [t(`powers.use.outcome.${c.outcome}`)];
  if (c.kept) {
    parts.push(t("powers.defence.keptPlusOne"));
    if (c.outcome !== "success" && c.outcome !== "natural_20")
      parts.push(t("powers.defence.failureAsOne"));
  }
  if (fromKept) parts.push(t("powers.defence.fromKept"));
  return `${t("powers.use.rolled", { d20: c.d20, target: c.target })} (${parts.join(", ")})`;
}
