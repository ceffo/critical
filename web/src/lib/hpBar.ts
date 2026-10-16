import { hpPercent, hpTone } from "./format";

/** What the HP bar draws, from the server's numbers only (display, no rules). */
export interface HpBarModel {
  /** Fill colour band (docs/frontend.md "Display Conventions"): good > 50 %, warn 25-50 %, bad < 25 %. */
  tone: ReturnType<typeof hpTone>;
  /** Width of the dying zone (floor..0) on the whole bar, in %; 0 when the floor is not below 0. */
  dyingPct: number;
  /** How much of the dying zone is filled from 0 down to current HP, in % of the zone. */
  deepPct: number;
  /** The fill, from 0 to current HP, in % of the 0..max part. */
  fillPct: number;
  /** The last damage taken, drawn after the fill, in % of the 0..max part. */
  ghostPct: number;
}

/**
 * The HP bar's geometry and colour band (ADR 0022 step 2.3, ADR 0011's floor): the scale runs from
 * the server's `floor` (-10) to `max`, the dying zone below 0 on the left, the fill from 0 to the
 * current HP coloured by threshold, and the last damage as a ghost after the fill.
 */
export function hpBarModel(
  current: number,
  max: number,
  floor: number,
  lastDamage: number,
): HpBarModel {
  const fillPct = hpPercent(current, max);
  const span = max - floor;
  return {
    tone: hpTone(fillPct),
    dyingPct: span > 0 && floor < 0 ? (-floor / span) * 100 : 0,
    deepPct:
      current < 0 && floor < 0 ? Math.min(100, (current / floor) * 100) : 0,
    fillPct,
    ghostPct: Math.max(0, hpPercent(current + lastDamage, max) - fillPct),
  };
}
