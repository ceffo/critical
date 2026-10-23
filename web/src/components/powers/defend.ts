// What the defend form (ADR 0044 step 8) sends and the one thing it checks before the server does.
// The contests, the checks and the winners are the server's; the current defence shown beside the
// form is the same pick the server makes (the mode raised this round, else the standing one).
import type { IncomingRoll } from "@/api/psionics";
import type { BuffEffect } from "@/types/buffs";

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

/** The character's current defence among the buff rows: raised this round, else standing. */
export function currentDefence(buffs: BuffEffect[]): BuffEffect | null {
  return (
    buffs.find((b) => b.defence === "raised") ??
    buffs.find((b) => b.defence === "standing") ??
    null
  );
}

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
