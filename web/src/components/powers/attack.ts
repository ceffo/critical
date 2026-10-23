// What the attack form (ADR 0044 step 7) sends and the checks it makes before the server does. The
// checks, the contests, the tangents and the cost are the server's.
import type { AttackRequest } from "@/api/psionics";
import { MAX_MODIFIER, MAX_MODIFIERS } from "./powerUse";

export const MAX_TARGET_LEN = 60;

export interface AttackDraft {
  target: string;
  mind: "open" | "closed";
  level: string;
  /** A defence mode's id, or "" for an undefended mind. */
  defence: string;
  /** The DM's roll for each of the two contests, entered with the defence. */
  rolls: { roll: string; success: boolean }[];
  modifiers: { value: string; reason: string }[];
}

export const EMPTY_ATTACK: AttackDraft = {
  target: "",
  mind: "closed",
  level: "",
  defence: "",
  rolls: [
    { roll: "", success: false },
    { roll: "", success: false },
  ],
  modifiers: [],
};

const whole = (s: string): number | null =>
  /^[+-]?\d+$/.test(s.trim()) ? Number.parseInt(s.trim(), 10) : null;

/** The request the draft makes, or null while the target, the level, a roll or a modifier is not valid. */
export function attackRequest(d: AttackDraft): AttackRequest | null {
  const target = d.target.trim();
  if (!target || target.length > MAX_TARGET_LEN) return null;
  const level = d.level.trim() === "" ? 0 : whole(d.level);
  if (level === null || level < 0) return null;
  const req: AttackRequest = { target, mind: d.mind, level, modifiers: [] };
  if (d.defence) {
    req.defence = d.defence;
    req.rolls = [];
    for (const r of d.rolls) {
      const n = whole(r.roll);
      if (n === null || n < 1 || n > 20) return null;
      req.rolls.push({ roll: n, success: r.success });
    }
  }
  if (d.modifiers.length > MAX_MODIFIERS) return null;
  for (const m of d.modifiers) {
    if (m.value.trim() === "") continue;
    const n = whole(m.value);
    if (n === null || Math.abs(n) > MAX_MODIFIER) return null;
    if (n !== 0) req.modifiers.push({ value: n, reason: m.reason.trim() });
  }
  return req;
}
