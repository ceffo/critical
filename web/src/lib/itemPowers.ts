// Display of the carried items' powers (docs/adr/0016-inventory.md steps 13 and 14). Which powers are
// on, and what they add to a roll, is the server's: this only names them from their item and words
// their effects.
import type { TFunction } from "i18next";
import type { BuffEffects, ItemPower } from "@/types/buffs";
import type { Item } from "@/types/items";
import { localized } from "@/i18n/localized";
import { formatAc, formatSigned } from "./format";

/** The current name of the item with an id, undefined when no item has it (removed since). */
export type ItemNamer = (itemId: string) => string | undefined;

/** Names items by id from the item list, in the UI language `lang` (ADR 0023 step 12). */
export function itemNamer(items: readonly Item[], lang: string): ItemNamer {
  return (itemId) => {
    const item = items.find((i) => i.id === itemId);
    return item ? localized(lang, item.name) : undefined;
  };
}

/**
 * A breakdown entry's detail (ADR 0016 step 14): an item power's contribution is named from its item
 * by id, so it shows the item's current name; without an item id, or when no item has it any more,
 * the detail as stored.
 */
export function namedDetail(
  m: { detail?: string; itemId?: string },
  itemName?: ItemNamer,
): string | undefined {
  return (m.itemId && itemName?.(m.itemId)) || m.detail;
}

/** One row of the effects timeline for an item power that is on. */
export interface PowerRow {
  /** Stable React key: item and power id. */
  key: string;
  itemId: string;
  /** The item's current name in the UI language (its id when no item has it). */
  name: string;
  /** The power's trigger (effective STR it needs), when the item list says it has one. */
  minStr?: number;
  effects: BuffEffects;
}

/**
 * The timeline rows of the item powers that are on, in the server's order, each under its item's
 * name (ADR 0016 step 13). A power the server does not list (below its trigger) has no row.
 */
export function powerRows(
  powers: readonly ItemPower[],
  items: readonly Item[],
  lang: string,
): PowerRow[] {
  return powers.map((p) => {
    const item = items.find((i) => i.id === p.itemId);
    return {
      key: `${p.itemId}/${p.powerId}`,
      itemId: p.itemId,
      name: item ? localized(lang, item.name) : p.itemId,
      minStr: item?.powers?.find((d) => d.id === p.powerId)?.minStr,
      effects: p.effects,
    };
  });
}

/**
 * A power's effects in words: "weapon +5 · dice ×2". The weapon +N and the dice
 * factor first (the Power Gem's), then the other effects.
 */
export function powerEffectsText(t: TFunction, e: BuffEffects): string {
  const parts: string[] = [];
  if (e.weapon_buff !== undefined)
    parts.push(t("buffs.effWeapon", { n: e.weapon_buff }));
  if (e.multiplier !== undefined)
    parts.push(t("buffs.effDice", { n: e.multiplier }));
  if (e.spells_buff !== undefined)
    parts.push(t("result.cast.fx.spells_buff", { value: e.spells_buff }));
  for (const key of ["to_hit", "damage", "save_bonus", "ac"] as const) {
    const v = e[key];
    if (v !== undefined)
      parts.push(t(`result.cast.fx.${key}`, { value: formatSigned(v) }));
  }
  if (e.base_ac !== undefined)
    parts.push(t("result.cast.fx.base_ac", { value: formatAc(e.base_ac) }));
  return parts.join(" · ");
}

/** A power as stored (itemsvc jsonPower: the item_edit change values, ADR 0033 step 17). */
interface StoredPower {
  id?: string;
  trigger?: { min_str?: number };
  effects?: BuffEffects;
}

/**
 * One power in words: its effects, then its conditions and trigger ("Saves +2 · AC −2, without
 * magic armour"). An armour's or a shield's "+N" (id magic) reads "+N".
 */
export function powerText(
  t: TFunction,
  p: { id?: string; minStr?: number; effects: BuffEffects },
): string {
  if (p.id === "magic" && p.effects.ac !== undefined && p.minStr === undefined)
    return t("equipment.powers.plusText", {
      value: formatSigned(-p.effects.ac),
    });
  const conditions: string[] = [];
  if (p.effects.weapon_kind === "melee")
    conditions.push(t("equipment.powers.meleeOnly"));
  if (p.effects.weapon_kind === "missile")
    conditions.push(t("equipment.powers.missileOnly"));
  if (p.effects.without_magic_armour)
    conditions.push(t("equipment.powers.withoutMagicArmour"));
  if (p.minStr !== undefined)
    conditions.push(t("result.cast.fx.min_str", { value: p.minStr }));
  const effects = powerEffectsText(t, p.effects);
  return conditions.length > 0
    ? t("equipment.powers.withConditions", {
        effects,
        conditions: conditions.join(", "),
      })
    : effects;
}

/**
 * An item_edit change value of the powers field (the stored encoding, ADR 0033 step 17) in words:
 * each power, "none" for an empty list; the text as logged when it is not a list.
 */
export function powersChangeText(t: TFunction, value: string): string {
  let list: unknown;
  try {
    list = JSON.parse(value);
  } catch {
    return value;
  }
  if (!Array.isArray(list)) return value;
  if (list.length === 0) return t("equipment.powers.noneShort");
  return (list as StoredPower[])
    .map((p) =>
      powerText(t, {
        id: p.id,
        minStr: p.trigger?.min_str,
        effects: p.effects ?? {},
      }),
    )
    .join(" ; ");
}
