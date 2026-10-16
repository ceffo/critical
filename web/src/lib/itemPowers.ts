// Display of the carried items' powers (docs/adr/0016-inventory.md steps 13 and 14). Which powers are
// on, and what they add to a roll, is the server's: this only names them from their item and words
// their effects.
import type { TFunction } from "i18next";
import type { BuffEffects, ItemPower } from "@/types/buffs";
import type { Item } from "@/types/items";
import { formatSigned } from "./format";

/** The current name of the item with an id, undefined when no item has it (removed since). */
export type ItemNamer = (itemId: string) => string | undefined;

/** Names items by id from the item list. */
export function itemNamer(items: readonly Item[]): ItemNamer {
  return (itemId) => items.find((i) => i.id === itemId)?.name;
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
  /** The item's current name (its id when no item has it). */
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
): PowerRow[] {
  return powers.map((p) => {
    const item = items.find((i) => i.id === p.itemId);
    return {
      key: `${p.itemId}/${p.powerId}`,
      itemId: p.itemId,
      name: item?.name ?? p.itemId,
      minStr: item?.powers?.find((d) => d.id === p.powerId)?.minStr,
      effects: p.effects,
    };
  });
}

/**
 * A power's effects in words: "weapon +5 · dice ×2 · melee only". The weapon +N and the dice
 * factor first (the Power Gem's), then the other effects, then the weapon-kind condition.
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
  if (e.weapon_kind === "melee") parts.push(t("buffs.effMelee"));
  if (e.weapon_kind === "missile") parts.push(t("buffs.effMissile"));
  return parts.join(" · ");
}
