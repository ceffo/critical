// Words shared by several entry kinds and by other screens: names from ids, counters, modifier
// labels. Display formatting only; every number is the server's.
import type { TFunction } from "i18next";
import { formatSigned, formatStrength } from "@/lib/format";
import { namedDetail, type ItemNamer } from "@/lib/itemPowers";

/** A skill's display name; an id without a translation shows as is. */
export function skillName(t: TFunction, id: string): string {
  return t(`skills.names.${id}`, { defaultValue: id });
}

/** The undead type's name (PHB Table 61 row code). */
export function undeadName(t: TFunction, code: string): string {
  return t(`turning.undead.${code}`, { defaultValue: code });
}

/** "DEX 17 −1" (the modifier left out when 0). */
export function abilityText(
  t: TFunction,
  ability: string,
  score: number,
  modifier: number,
): string {
  const name = t(`attr.${ability}`, { defaultValue: ability });
  return modifier === 0
    ? `${name} ${score}`
    : `${name} ${score} ${formatSigned(modifier)}`;
}

/** "5 rd, 3 ch", or "until dismissed" when the buff has neither counter. */
export function counters(t: TFunction, rounds = 0, charges = 0): string {
  const parts = [
    rounds > 0 ? t("buffs.rounds", { count: rounds }) : null,
    charges > 0 ? t("buffs.charges", { count: charges }) : null,
  ].filter((p): p is string => p !== null);
  return parts.length > 0 ? parts.join(", ") : t("buffs.untilDismissed");
}

/** A multiplier inside a formula, with at least one decimal: 2.0, 1.5, 1.25 (1,5 in French). */
export function formatMultiplier(n: number, lang?: string): string {
  return new Intl.NumberFormat(lang, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 4,
  }).format(n);
}

/** "+ 3" / "− 3" for a term added at the end of a formula. */
export function plusTerm(n: number): string {
  return n < 0 ? ` − ${Math.abs(n)}` : ` + ${n}`;
}

/**
 * A modifier's or damage term's label in the player's language (detail = score, weapon, ammo or
 * buff name; an item power's is named from its item by id, ADR 0016 step 14). The crit term is
 * labelled with the roll's crit-table range (none: the default placeholder table); an unnamed
 * situational modifier gets the generic word.
 */
export function modLabel(
  t: TFunction,
  m: {
    kind: string;
    detail?: string;
    itemId?: string;
    value: number;
    strExceptional?: number;
  },
  roll?: { critPlaceholder?: boolean; critFrom?: number; critTo?: number },
  itemName?: ItemNamer,
): string {
  if (m.kind === "crit") {
    return roll && !roll.critPlaceholder && roll.critFrom
      ? t("result.mods.critRange", { from: roll.critFrom, to: roll.critTo })
      : t("result.mods.critPlaceholder");
  }
  if (m.kind === "situational" && !m.detail)
    return t("result.mods.situationalUnnamed");
  // The race's save bonus (ADR 0030 step 22.1): the detail is the race code.
  if (m.kind === "race")
    return t("result.mods.race", {
      race: t(`details.races.${m.detail ?? ""}`, { defaultValue: m.detail }),
    });
  // An 18/xx Strength (ADR 0030 step 9): the score and its percentile.
  if (m.kind === "strength" && m.strExceptional)
    return t("result.mods.strength", {
      detail: formatStrength(Number(m.detail), m.strExceptional),
    });
  return t(`result.mods.${m.kind}`, {
    detail: namedDetail(m, itemName) ?? "",
    value: m.value,
  });
}
