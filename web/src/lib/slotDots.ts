// The slot dots of one spell level on the Spells tab (ADR 0027 step 10.3), from the server's slot
// data alone: no rule is decided here, the server says which slots exist, which are used and how many
// can be prepared at the level.
import type { SpellSlot } from "@/types/spells";

/**
 * One dot: a slot that can still be cast, a slot spent today, or a slot the level allows with no
 * spell prepared in it.
 */
export type DotState = "available" | "used" | "empty";

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
