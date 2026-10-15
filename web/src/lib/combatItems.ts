import type { Item } from "@/types/items";

/** The items the Agir Items tab lists: everything but ammunition (fired from the Weapons tab). */
export function combatItems(items: Item[]): Item[] {
  return items.filter((i) => i.ammoKind === "");
}
