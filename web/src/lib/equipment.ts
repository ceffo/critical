// Presentation of the possessions (ADR 0033, ADR 0035): which pieces a category chip shows, whether
// a piece shows as Equipped, and the stats line of its row. Every number and code comes from the
// server; this only words and orders them.
import type { TFunction } from "i18next";
import type { Category, Item } from "@/types/items";
import { formatSigned } from "@/lib/format";

/** Equipped is the one state word the player sees, for worn and wielded alike (ADR 0035 item 3). */
export function equipped(i: Pick<Item, "state">): boolean {
  return i.state !== "carried";
}

/** The pieces a chip shows, in the player's order (ADR 0035 item 2: the chips are the only filter). */
export function piecesIn(items: Item[], filter: Category | "all"): Item[] {
  return filter === "all" ? items : items.filter((i) => i.category === filter);
}

/** One part of a row's stats line; the behaviour part carries its hint. */
export interface StatPart {
  text: string;
  behaviour?: boolean;
}

/**
 * The stats line of a piece's row, by category (ADR 0033 steps 3 to 5): a weapon's kind, damage,
 * magic and speed; ammunition's kind, damage and magic; an armour's base AC, skills column and
 * type; a shield's kind and bonus; a magic piece's kind; and the count behaviour of what is counted.
 */
export function pieceStats(i: Item, t: TFunction): StatPart[] {
  const parts: StatPart[] = [];
  const add = (text: string | false | undefined | null) => {
    if (text) parts.push({ text });
  };
  const behaviour = () =>
    parts.push({ text: t(`items.behaviour.${i.behaviour}`), behaviour: true });
  const damage = () => {
    if (i.damageSm) add(i.damageSm);
    if (i.damageL && i.damageL !== i.damageSm)
      add(t("equipment.stats.large", { dice: i.damageL }));
    if (i.damageTypes.length > 0)
      add(
        i.damageTypes
          .map((d) => t(`equipment.damageTypesShort.${d}`))
          .join("/"),
      );
  };
  switch (i.category) {
    case "weapon":
      if (i.weaponKind) add(t(`weapons.kind.${i.weaponKind}`));
      damage();
      if (i.magicToHit !== 0 || i.magicDamage !== 0)
        add(
          t("equipment.stats.magic", {
            hit: formatSigned(i.magicToHit),
            dmg: formatSigned(i.magicDamage),
          }),
        );
      if (i.speed > 0) add(t("equipment.stats.speed", { speed: i.speed }));
      if (i.twoHanded) add(t("equipment.stats.twoHanded"));
      if (i.specialized) add(t("weapons.specialized"));
      if (i.fires)
        add(
          t("equipment.stats.fires", {
            kind: t(`equipment.ammoKinds.${i.fires}`, {
              defaultValue: i.fires,
            }),
          }),
        );
      break;
    case "ammunition":
      add(t(`equipment.ammoKinds.${i.ammoKind}`, { defaultValue: i.ammoKind }));
      damage();
      behaviour();
      break;
    case "armour":
      add(t("equipment.stats.ac", { ac: i.baseAc }));
      if (i.skillColumn)
        add(
          t("equipment.stats.skills", {
            column: t(`equipment.skillColumns.${i.skillColumn}`),
          }),
        );
      if (i.armorRow)
        add(
          t("equipment.stats.type", {
            row: t(`equipment.armorRows.${i.armorRow}`),
          }),
        );
      break;
    case "shield":
      if (i.shieldKind) add(t(`equipment.shieldKinds.${i.shieldKind}`));
      add(
        i.acMissile !== i.acMelee
          ? t("equipment.stats.shieldMissile", {
              melee: i.acMelee,
              missile: i.acMissile,
            })
          : t("equipment.stats.shield", { melee: i.acMelee }),
      );
      break;
    case "magic":
      if (i.magicKind) add(t(`equipment.magicKinds.${i.magicKind}`));
      behaviour();
      break;
    case "gear":
      behaviour();
      break;
  }
  return parts;
}
