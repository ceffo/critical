// The inventory entries (itemsvc and weaponsvc records, ADR 0016, ADR 0033 step 7): restock, the
// editor's add / edit / remove of an item or a weapon, and a piece equipped or unequipped. Each
// changes state and rolls nothing.
import type { TFunction } from "i18next";
import { Package, Sword, TShirt } from "@phosphor-icons/react";
import type { InventoryNamer } from "@/i18n/inventoryName";
import type {
  EquipmentStateRecord,
  ItemChangeRecord,
  RestockResult,
} from "@/types/items";
import type { WeaponChangeRecord } from "@/types/combat";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { powersChangeText } from "@/lib/itemPowers";

/** Payload of item_restock entries (itemsvc restockRecord, same shape as RestockResult). */
function decodeRestock(e: LogEntry): RestockResult | null {
  const p = e.payload as Partial<RestockResult> | null;
  return p &&
    typeof p.name === "string" &&
    typeof p.added === "number" &&
    typeof p.count === "number"
    ? (p as RestockResult)
    : null;
}

export const itemRestockKind: KindView<RestockResult> = {
  decode: decodeRestock,
  icon: Package,
  text: (p, { t, names }) =>
    t("log.item.restocked", {
      name: names.item(p.itemId, p.name),
      added: p.added,
      count: p.count,
    }),
};

/** Payload of item_add, item_edit and item_remove entries (itemsvc changeRecord). */
function decodeItemChange(e: LogEntry): ItemChangeRecord | null {
  const p = e.payload as Partial<ItemChangeRecord> | null;
  return p && typeof p.name === "string" && typeof p.count === "number"
    ? (p as ItemChangeRecord)
    : null;
}

function itemEditText(
  p: ItemChangeRecord,
  t: TFunction,
  names: InventoryNamer,
): string {
  const changes = (p.changes ?? [])
    .map((c) =>
      t("log.item.change", {
        ...c,
        // The powers change holds the stored lists (ADR 0033 step 17): worded here.
        ...(c.field === "powers"
          ? {
              before: powersChangeText(t, c.before),
              after: powersChangeText(t, c.after),
            }
          : {}),
        field: t(`log.item.fields.${c.field}`, { defaultValue: c.field }),
      }),
    )
    .join(", ");
  return t("log.item.edited", { name: names.item(p.itemId, p.name), changes });
}

export const itemAddKind: KindView<ItemChangeRecord> = {
  decode: decodeItemChange,
  icon: Package,
  text: (p, { t, names }) =>
    t("log.item.added", { name: names.item(p.itemId, p.name), count: p.count }),
};

export const itemEditKind: KindView<ItemChangeRecord> = {
  decode: decodeItemChange,
  icon: Package,
  text: (p, { t, names }) => itemEditText(p, t, names),
};

export const itemRemoveKind: KindView<ItemChangeRecord> = {
  decode: decodeItemChange,
  icon: Package,
  text: (p, { t, names }) =>
    t("log.item.removedItem", {
      name: names.item(p.itemId, p.name),
      count: p.count,
    }),
};

/** Payload of weapon_add, weapon_edit and weapon_remove entries (weaponsvc changeRecord), written
 * before the one equipment model (ADR 0033 step 26.2): still shown and popped. */
function decodeWeaponChange(e: LogEntry): WeaponChangeRecord | null {
  const p = e.payload as Partial<WeaponChangeRecord> | null;
  return p && typeof p.name === "string" ? (p as WeaponChangeRecord) : null;
}

function weaponEditText(
  p: WeaponChangeRecord,
  t: TFunction,
  names: InventoryNamer,
): string {
  const changes = (p.changes ?? [])
    .map((c) =>
      t("log.weapon.change", {
        ...c,
        field: t(`log.weapon.fields.${c.field}`, { defaultValue: c.field }),
      }),
    )
    .join(", ");
  return t("log.weapon.edited", {
    name: names.weapon(p.weapon_id, p.name),
    changes,
  });
}

export const weaponAddKind: KindView<WeaponChangeRecord> = {
  decode: decodeWeaponChange,
  icon: Sword,
  text: (p, { t, names }) =>
    t("log.weapon.added", { name: names.weapon(p.weapon_id, p.name) }),
};

export const weaponEditKind: KindView<WeaponChangeRecord> = {
  decode: decodeWeaponChange,
  icon: Sword,
  text: (p, { t, names }) => weaponEditText(p, t, names),
};

export const weaponRemoveKind: KindView<WeaponChangeRecord> = {
  decode: decodeWeaponChange,
  icon: Sword,
  text: (p, { t, names }) =>
    t("log.weapon.removed", { name: names.weapon(p.weapon_id, p.name) }),
};

/**
 * Payload of equipment_state entries (itemsvc stateRecord, ADR 0033 step 7), worded with the one
 * term of ADR 0035 item 3: "Equipped: X", "Unequipped: X", a swap "Equipped: X (in place of Y)".
 */
function decodeEquipmentState(e: LogEntry): EquipmentStateRecord | null {
  const p = e.payload as Partial<EquipmentStateRecord> | null;
  return p && typeof p.name === "string" ? (p as EquipmentStateRecord) : null;
}

export const equipmentStateKind: KindView<EquipmentStateRecord> = {
  decode: decodeEquipmentState,
  icon: TShirt,
  text(p, { t, names }) {
    const name = names.item(p.itemId, p.name);
    if (p.after === "carried") return t("log.equipment.unequipped", { name });
    const off = (p.off ?? []).map((o) => names.item(o.itemId, o.name));
    if (off.length > 0)
      return t("log.equipment.swapped", { name, off: off.join(", ") });
    return t("log.equipment.equipped", { name });
  },
};
