import type { TFunction } from "i18next";
import type {
  AbilityUseResult,
  Item,
  ItemAbility,
  UseResult,
} from "@/types/items";

/** One row of Act · Items: an ability and the item that holds it (ADR 0049 step 14). */
export interface AbilityRow {
  item: Item;
  ability: ItemAbility;
}

/**
 * The rows of Act · Items, in the item list order: one per ability of every item that has some
 * (ADR 0049 step 14). An item with none is never listed, so neither are the weapons, armour and
 * shields without an ability; ammunition is fired from the Weapons tab. Whether a row can be used
 * is the ability's `usable`, served by the server.
 */
export function abilityRows(items: Item[]): AbilityRow[] {
  return items
    .filter((i) => i.ammoKind === "")
    .flatMap((item) =>
      (item.abilities ?? []).map((ability) => ({ item, ability })),
    );
}

/**
 * The ability use's answer in the shape of a logged `item_use` entry (the server's flat fields are
 * the first dice effect): the result card and the journal read both the same way.
 */
export function loggedAbilityUse(r: AbilityUseResult): UseResult {
  const first = r.effects[0];
  return {
    itemId: r.itemId,
    name: r.name,
    type: r.type,
    removed: r.removed,
    remaining: r.remaining,
    maxCount: r.max ?? 0,
    dice: first?.dice ?? "",
    rolls: first?.rolls ?? [],
    modifier: first?.modifier,
    total: first?.total ?? 0,
    save: first?.save ?? "",
    notes: r.notes ?? "",
    logEntryId: r.logEntryId,
    abilityId: r.abilityId,
    abilityName: r.abilityName,
    limit: r.limit,
    left: r.left,
    max: r.max,
    effects: r.effects,
  };
}

/**
 * An ability's limit as the player reads it (ADR 0049 step 14): `∞`, `1 / 2 per day`, `3 / 10
 * charges` (with the cost of a use when it is more than one), `× 3` for the item's own quantity.
 * The numbers are the server's; this only words them. Shared by Act · Items and the Inventory.
 */
export function limitText(
  t: TFunction,
  ability: Pick<ItemAbility, "limit">,
  quantity: number,
): string {
  const { kind, left = 0, max = 0, cost = 1 } = ability.limit;
  return kind === "charges" && cost > 1
    ? t("actions.items.limit.chargesCost", { left, max, cost })
    : t(`actions.items.limit.${kind}`, { left, max, count: quantity });
}
