// The slot dots of one spell level on the Spells tab (ADR 0027 step 10.3), from the server's slot
// data alone: no rule is decided here, the server says which slots exist, which are used and how many
// can be prepared at the level.
import type { SpellSlot } from "@/types/spells";

/**
 * One dot: a slot that can still be cast, a slot spent today, a slot the level allows with no
 * spell prepared in it, or (in the preparation editor) a spell prepared beyond what the level allows.
 */
export type DotState = "available" | "used" | "empty" | "over";

export interface SlotDots {
  dots: DotState[];
  available: number;
  used: number;
  empty: number;
}

/**
 * The dots of a level: the available slots, then the used ones, then one empty dot per preparable
 * slot left unprepared (`max` from the server's limits, minus the prepared slots). Extra slots a
 * spell granted (Rary's Mnemonic Enhancer) are slots of their own: available until cast, used after,
 * never counted against `max`.
 */
export function slotDots(
  slots: SpellSlot[],
  max: number | undefined,
): SlotDots {
  const used = slots.filter((s) => s.used).length;
  const available = slots.length - used;
  const prepared = slots.filter((s) => !s.extra).length;
  const empty = Math.max(0, (max ?? 0) - prepared);
  return {
    dots: [
      ...Array<DotState>(available).fill("available"),
      ...Array<DotState>(used).fill("used"),
      ...Array<DotState>(empty).fill("empty"),
    ],
    available,
    used,
    empty,
  };
}

/**
 * The preparation editor's dots of a level (ADR 0027 step 11.5), in the Spells tab's language: one
 * filled dot per spell prepared within `max` (the server's count), an outline per slot left, then
 * one "over" dot per spell beyond it.
 */
export function preparedDots(count: number, max: number): DotState[] {
  const within = Math.min(count, max);
  return [
    ...Array<DotState>(within).fill("available"),
    ...Array<DotState>(Math.max(0, max - count)).fill("empty"),
    ...Array<DotState>(Math.max(0, count - max)).fill("over"),
  ];
}
