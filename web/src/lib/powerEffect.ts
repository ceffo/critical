// A power's effect in words (ADR 0043 step 17). Display only: every number is the catalogue's and
// the rule that applies it is the server's (docs/all.md, "Core Architectural Rule").
import type { TFunction } from "i18next";
import type { PowerEffect } from "@/types/powers";

/** The effect keys a power can carry, in the order the lookup lists them. */
const KEYS = [
  "ac",
  "to_hit",
  "damage",
  "save_bonus",
  "str_bonus",
  "initiative",
] as const;

/** One line of the effect: the words for the key with its signed number. */
export interface PowerEffectLine {
  key: string;
  text: string;
}

/** A signed number with a true minus (an AC of −2 betters the armour class). */
const signed = (n: number) => (n < 0 ? `−${-n}` : `+${n}`);

/**
 * The lines of a power's effect, then the condition (the weapon kind) when the power has one. Empty when the power has no effect.
 */
export function powerEffectView(
  t: TFunction,
  effect: PowerEffect | undefined,
): { lines: PowerEffectLine[]; notes: string[] } {
  if (!effect) return { lines: [], notes: [] };
  const lines = KEYS.flatMap((key) => {
    const n = effect[key];
    return n === undefined || n === 0
      ? []
      : [
          {
            key,
            text: t(`lookup.powers.effectLines.${key}`, { value: signed(n) }),
          },
        ];
  });
  const notes = [
    effect.weapon_kind
      ? t(`lookup.powers.effectWeaponKinds.${effect.weapon_kind}`)
      : "",
  ].filter(Boolean);
  return { lines, notes };
}
