// Wording of the damage dice the server evaluated at a caster level; no game rules here.
import type { TFunction } from "i18next";
import type { DiceSum, SpellDamage, StepPer } from "@/types/spells";

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

/** True when a sum rolls nothing: no dice and 0 (a dice step without damage). */
function isNone(sum: DiceSum): boolean {
  return sum.groups.length === 0 && sum.modifier === 0;
}

/**
 * The roll of one instance as a dice composer formula ("1d4+1"); the server parses it. A damage
 * made of dice steps sends its first step with damage.
 */
export function damageFormula(d: SpellDamage): string {
  const step = d.steps?.find((s) => !isNone(s.dice));
  return diceText(step ? step.dice : d.dice);
}

/** The label of one damage roll of a cast: "Turn 3" for a step per round, else "#n". */
export function rollLabel(t: TFunction, stepsPer: StepPer | undefined, step: number | undefined, index: number): string {
  if (stepsPer === "round") return t("result.cast.round", { n: step ?? index + 1 });
  return t("result.cast.instance", { n: step ?? index + 1 });
}

/** The header of a cast's damage made of dice steps: "5 turns, one roll each", "1 roll". */
export function stepsHeader(t: TFunction, stepsPer: StepPer, count: number): string {
  return t(`result.cast.damageSteps.${stepsPer}`, { count });
}

/** Each dice step in order, "—" for one without damage: "by round: —, —, 20d2, 20d4, 20d2". */
function stepsText(t: TFunction, d: SpellDamage): string {
  const steps = (d.steps ?? []).map((s) => (isNone(s.dice) ? "—" : diceText(s.dice))).join(", ");
  return t(`spellDice.steps.${d.steps_per ?? "instance"}`, { steps });
}

/**
 * The dice of one instance: "1d4+1", or its parts with what a save does to each
 * ("2d4 [no save] + 15 [save negates]"), in parentheses when repeated.
 */
function instanceText(t: TFunction, d: SpellDamage): string {
  if (!d.parts || d.parts.length === 0) return diceText(d.dice);
  const parts = d.parts
    .map((p) => t(`spellDice.parts.${p.save}`, { dice: diceText(p.dice), defaultValue: diceText(p.dice) }))
    .join(" + ");
  return d.parts.length > 1 && !isOne(d.instances) ? `(${parts})` : parts;
}

/**
 * "10d6 fire", "5 × 1d4+1 force", "2d4 acid per round, 6 rounds", "by round: 12d4, 10d4 acid"
 * (each dice step), in the player's language.
 */
export function damageText(t: TFunction, d: SpellDamage): string {
  const type = d.type ? t(`spellDice.types.${d.type}`, { defaultValue: d.type }) : "";
  if (d.steps && d.steps.length > 0) {
    const steps = stepsText(t, d);
    return type ? t("spellDice.typed", { dice: steps, type }) : steps;
  }
  let dice = instanceText(t, d);
  if (!isOne(d.instances)) {
    dice = t("spellDice.instances", { instances: diceText(d.instances), dice });
  }
  let text = type ? t("spellDice.typed", { dice, type }) : dice;
  if (!isOne(d.rounds)) {
    text = t("spellDice.perRound", { text, rounds: diceText(d.rounds) });
  }
  return text;
}

/**
 * One round of exposure's damage (ADR 0008 step 16): "1d10 poison", or for dice steps, each round
 * of exposure's then the last one's for the rounds after ("1, 2, 4, 8 acid by round of exposure,
 * then 8 each round").
 */
export function exposureDamageText(t: TFunction, d: SpellDamage): string {
  if (!d.steps || d.steps.length === 0) return damageText(t, d);
  let steps = d.steps.map((s) => (isNone(s.dice) ? "—" : diceText(s.dice))).join(", ");
  const type = d.type ? t(`spellDice.types.${d.type}`, { defaultValue: d.type }) : "";
  if (type) steps = t("spellDice.typed", { dice: steps, type });
  return t("spellDice.exposureSteps", { steps, last: diceText(d.steps[d.steps.length - 1].dice) });
}
