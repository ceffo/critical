import type { Item } from "@/types/items";

/**
 * The items the Agir Items tab lists: what the server says can be used (not a weapon, an armour or
 * a shield) and is not ammunition (fired from the Weapons tab).
 */
export function combatItems(items: Item[]): Item[] {
  return items.filter((i) => i.usable && i.ammoKind === "");
}
