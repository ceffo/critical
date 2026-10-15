// Wording of the damage dice the server evaluated at a caster level; no game rules here.
import type { TFunction } from "i18next";
import type { DiceSum, SpellDamage } from "@/types/spells";

/** "10d6", "1d4+1", "1d6+4d6", "5": the server's groups then its constant, as a dice formula. */
export function diceText(sum: DiceSum): string {
  const dice = sum.groups.map((g) => `${g.count}d${g.sides}`).join("+");
  if (!dice) return `${sum.modifier}`;
  if (sum.modifier === 0) return dice;
  return `${dice}${sum.modifier > 0 ? "+" : ""}${sum.modifier}`;
}

/** True when the sum is exactly the number 1 (one instance, one round). */
function isOne(sum: DiceSum): boolean {
  return sum.groups.length === 0 && sum.modifier === 1;
}

/** The roll of one instance as a dice composer formula ("1d4+1"); the server parses it. */
export function damageFormula(d: SpellDamage): string {
  return diceText(d.dice);
}

/** "10d6 fire", "5 × 1d4+1 force", "2d4 acid per round, 6 rounds", in the player's language. */
export function damageText(t: TFunction, d: SpellDamage): string {
  let dice = diceText(d.dice);
  if (!isOne(d.instances)) {
    dice = t("spellDice.instances", { instances: diceText(d.instances), dice });
  }
  const type = d.type ? t(`spellDice.types.${d.type}`, { defaultValue: d.type }) : "";
  let text = type ? t("spellDice.typed", { dice, type }) : dice;
  if (!isOne(d.rounds)) {
    text = t("spellDice.perRound", { text, rounds: diceText(d.rounds) });
  }
  return text;
}
