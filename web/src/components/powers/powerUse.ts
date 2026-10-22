// What the use form of a psionic power (ADR 0043 steps 10, 11 and 13) asks and what it can refuse
// before the server does. Display and input only: the check, the cost and the refusals are the
// server's; the form words the two that are plain from the numbers on screen.
import type { UsePowerRequest } from "@/api/psionics";
import type { Power } from "@/types/powers";

/** The server's bounds (psionics.MaxModifier, psionicsvc.MaxModifiers, MaxEnteredScore). */
export const MAX_MODIFIER = 20;
export const MAX_MODIFIERS = 10;
export const MAX_ENTERED_SCORE = 25;
export const MAX_REASON_LEN = 80;

export interface UseDraft {
  cost: string;
  maintenanceCost: string;
  score: string;
  modifiers: { value: string; reason: string }[];
}

export const EMPTY_DRAFT: UseDraft = {
  cost: "",
  maintenanceCost: "",
  score: "",
  modifiers: [],
};

const whole = (s: string): number | null => {
  const t = s.trim();
  return /^[+-]?\d+$/.test(t) ? Number.parseInt(t, 10) : null;
};

/** Which inputs the power asks for. */
export function asks(p: Power) {
  return {
    cost: p.initial_cost === "variable",
    maintenanceCost: p.maintenance?.cost === "variable",
    score: !p.score,
  };
}

/**
 * The request the draft makes, or null while an input the power asks for is not a whole number in
 * range (a modifier left blank is dropped, a 0 adds nothing).
 */
export function requestOf(p: Power, d: UseDraft): UsePowerRequest | null {
  const need = asks(p);
  const req: UsePowerRequest = { modifiers: [] };
  if (need.cost) {
    const n = whole(d.cost);
    if (n === null || n < 1) return null;
    req.cost = n;
  }
  if (need.maintenanceCost) {
    const n = whole(d.maintenanceCost);
    if (n === null || n < 1) return null;
    req.maintenanceCost = n;
  }
  if (need.score) {
    const n = whole(d.score);
    if (n === null || n < 1 || n > MAX_ENTERED_SCORE) return null;
    req.score = n;
  }
  for (const m of d.modifiers) {
    if (m.value.trim() === "") continue;
    const n = whole(m.value);
    if (n === null || Math.abs(n) > MAX_MODIFIER) return null;
    if (n !== 0) req.modifiers.push({ value: n, reason: m.reason.trim() });
  }
  return req;
}
