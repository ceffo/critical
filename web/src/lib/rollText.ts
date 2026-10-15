// Display formatting only: the numbers come from the server as computed (docs/all.md, "Core
// Architectural Rule"); these functions lay them out, they never compute a game value.
import type { TFunction } from "i18next";
import type { RollResult } from "@/types/combat";
import { formatSigned } from "@/lib/format";
import type { UseResult } from "@/types/items";

/** A multiplier inside a formula, with at least one decimal: 2.0, 1.5, 1.25 (1,5 in French). */
export function formatMultiplier(n: number, lang?: string): string {
  return new Intl.NumberFormat(lang, { minimumFractionDigits: 1, maximumFractionDigits: 4 }).format(n);
}

/** "+ 3" / "− 3" for a term added at the end of a formula. */
function plusTerm(n: number): string {
  return n < 0 ? ` − ${Math.abs(n)}` : ` + ${n}`;
}

/**
 * The damage formula of a hit with its numbers (docs/adr/0003 step 2):
 * ((dice × spells [× dice multiplier]) + weapon) × crit + strength [± bonus] = total. A result the
 * server floored (or clamped at 0) shows its unrounded value first: "= 10.5 → 10".
 */
export function damageFormula(r: RollResult, lang?: string): string {
  const m = (n: number) => formatMultiplier(n, lang);
  let dice = `${r.baseDamage} × ${m(r.spellsBuff)}`;
  if (r.diceMultiplier !== undefined && r.diceMultiplier > 0 && r.diceMultiplier !== 1) {
    dice += ` × ${m(r.diceMultiplier)}`;
  }
  let s = `((${dice}) + ${r.weaponBuff}) × ${m(r.multipliers)} + ${r.strengthBonus}`;
  if (r.bonusDamage !== 0) s += plusTerm(r.bonusDamage);
  const unrounded = r.unroundedDamage;
  if (unrounded !== undefined && Math.abs(unrounded - r.totalDamage) > 1e-9) {
    return `${s} = ${new Intl.NumberFormat(lang, { maximumFractionDigits: 4 }).format(unrounded)} → ${r.totalDamage}`;
  }
  return `${s} = ${r.totalDamage}`;
}

/** An item's effect roll: "6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21", "1d8+2: 5 + 2 = 7". */
export function itemRollText(r: UseResult): string {
  let expr = r.rolls.join(" + ");
  if (r.modifier) expr += plusTerm(r.modifier);
  return `${r.dice}: ${expr} = ${r.total}`;
}

/**
 * A modifier's or damage term's label in the player's language (detail = score, weapon, ammo or
 * buff name). The crit term is labelled with the roll's crit-table range (none: the default
 * placeholder table); an unnamed situational modifier gets the generic word.
 */
export function modLabel(
  t: TFunction,
  m: { kind: string; detail?: string; value: number },
  roll?: { critPlaceholder?: boolean; critFrom?: number; critTo?: number },
): string {
  if (m.kind === "crit") {
    return roll && !roll.critPlaceholder && roll.critFrom
      ? t("result.mods.critRange", { from: roll.critFrom, to: roll.critTo })
      : t("result.mods.critPlaceholder");
  }
  if (m.kind === "situational" && !m.detail) return t("result.mods.situationalUnnamed");
  return t(`result.mods.${m.kind}`, { detail: m.detail ?? "", value: m.value });
}

/** A save's breakdown (docs/adr/0006 step 2): d20 + modifiers [+ adjustment] = total vs target. */
export function saveFormula(t: TFunction, r: RollResult, category: string): string {
  return t(r.adjustment ? "result.saveFormulaAdjusted" : "result.saveFormula", {
    d20: r.d20Roll,
    mod: r.saveBonus ?? 0,
    adj: formatSigned(r.adjustment ?? 0),
    total: r.saveTotal,
    target: r.effectiveThac0,
    name: t(`saves.categories.${category}`),
  });
}
