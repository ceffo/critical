// The words of an initiative's breakdown (ADR 0035 item 6.3): each modifier named by its source, in
// the player's language. Display only: the terms, their values and the total are the server's.
import type { TFunction } from "i18next";
import type { SpellNamer } from "@/i18n/spellName";
import type { InventoryNamer } from "@/i18n/inventoryName";
import type { InitiativeModifier } from "@/types/buffs";
import { formatSigned } from "./format";

/** The name of what a modifier comes from: the weapon, the buff or power, the spell, the item. */
export function initiativeTermName(
  m: InitiativeModifier,
  spellName: SpellNamer,
  names: InventoryNamer,
): string {
  const recorded = m.name ?? "";
  switch (m.source) {
    case "weapon":
      return names.weapon(m.itemId, recorded);
    case "item":
      return names.item(m.itemId, recorded);
    case "spell":
      return m.spellId ? spellName(m.spellId, recorded) : recorded;
    case "effect":
      if (m.itemId) return names.item(m.itemId, recorded);
      return m.spellId ? spellName(m.spellId, recorded) : recorded;
  }
}

/** One modifier as a line of the breakdown: "Long sword: +5", "Fly: end of round". */
export function initiativeTermText(
  m: InitiativeModifier,
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): string {
  const name = initiativeTermName(m, spellName, names);
  if (m.timing === "end_of_round")
    return t("initiative.endOfRoundTerm", { name });
  if (m.timing === "special") return t("initiative.specialTerm", { name });
  if (m.timing === "spell_casting_time")
    return t("initiative.spellCastingTimeTerm", { name });
  return t("initiative.term", { name, value: formatSigned(m.value) });
}

/** The breakdown in one line: "d10 7, Long sword: +5, Haste: −2". */
export function initiativeBreakdown(
  d10: number,
  modifiers: readonly InitiativeModifier[],
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): string {
  return [
    t("initiative.die", { value: d10 }),
    ...modifiers.map((m) => initiativeTermText(m, t, spellName, names)),
  ].join(", ");
}
