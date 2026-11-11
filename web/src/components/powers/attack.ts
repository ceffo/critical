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

/** A field the attack form still needs before it can send (the one condition: the request and the marks read it). */
export type AttackNeed = "target" | "level" | "rolls";

/**
 * What the draft lacks: the target, a readable level (empty is level 0, the mind and the defence are
 * defaults), and, with a defence up, the DM's two rolls (a whole number from 1 to 20).
 */
export function attackNeeds(d: AttackDraft): AttackNeed[] {
  const needs: AttackNeed[] = [];
  const target = d.target.trim();
  if (!target || target.length > MAX_TARGET_LEN) needs.push("target");
  const level = d.level.trim() === "" ? 0 : whole(d.level);
  if (level === null || level < 0) needs.push("level");
  if (
    d.defence &&
    d.rolls.some((r) => {
      const n = whole(r.roll);
      return n === null || n < 1 || n > 20;
    })
  )
    needs.push("rolls");
  return needs;
}

/** The request the draft makes, or null while a need (`attackNeeds`) or a modifier is not valid. */
export function attackRequest(d: AttackDraft): AttackRequest | null {
  if (attackNeeds(d).length > 0) return null;
  const target = d.target.trim();
  const level = d.level.trim() === "" ? 0 : whole(d.level)!;
  const req: AttackRequest = { target, mind: d.mind, level, modifiers: [] };
  if (d.defence) {
    req.defence = d.defence;
    req.rolls = [];
    for (const r of d.rolls) {
      req.rolls.push({ roll: whole(r.roll)!, success: r.success });
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
