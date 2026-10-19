// Presentation of the equipment catalogue in Ajouter > Catalogue (ADR 0033 step 24, ADR 0035 item
// 10): the rows of a category, the search over their names in the player's language, their cells,
// and the body that picks one. The server copies the row; nothing here decides a rule.
import type { TFunction } from "i18next";
import { localized, editLang } from "@/i18n/localized";
import type {
  EquipmentCatalogue,
  ItemBody,
  PickedCategory,
} from "@/types/items";
import type { LocalizedText } from "@/types/localized";

/** One catalogue row as the list shows it: its code, its name and its cells after the name. */
export interface CatalogueRow {
  code: string;
  name: LocalizedText;
  cells: string[];
}

/** Lower case, accents dropped: "Épée" finds "epee" and "épée". */
export function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase().trim();
}

/** The column headings after the name, per category (i18n keys under equipment.add.columns). */
export const COLUMNS: Record<PickedCategory, string[]> = {
  weapon: ["damage", "speed", "kind"],
  ammunition: ["ammoKind", "damage"],
  armour: ["ac", "skills", "armorRow"],
  shield: ["shieldKind", "melee", "missile"],
};

/** A damage as printed, S-M then large ("1d8 · G 1d12"), "—" for none (a launcher). */
function damage(t: TFunction, sm: string, l: string): string {
  if (!sm && !l) return "—";
  return l && l !== sm
    ? `${sm} · ${t("equipment.stats.large", { dice: l })}`
    : sm;
}

/** The rows of `category` whose name in `lang` holds `query` (accents and case ignored), in the catalogue's order. */
export function catalogueRows(
  c: EquipmentCatalogue,
  category: PickedCategory,
  query: string,
  lang: string,
  t: TFunction,
): CatalogueRow[] {
  const rows: CatalogueRow[] = (() => {
    switch (category) {
      case "weapon":
        return c.weapons.map((w) => ({
          code: w.code,
          name: w.name,
          cells: [
            damage(t, w.damageSm, w.damageL),
            String(w.speed),
            t(`weapons.kind.${w.weaponKind}`),
          ],
        }));
      case "ammunition":
        return c.ammunition.map((a) => ({
          code: a.code,
          name: a.name,
          cells: [
            t(`equipment.ammoKinds.${a.ammoKind}`, {
              defaultValue: a.ammoKind,
            }),
            damage(t, a.damageSm, a.damageL),
          ],
        }));
      case "armour":
        return c.armour.map((a) => ({
          code: a.code,
          name: a.name,
          cells: [
            String(a.baseAc),
            t(`equipment.skillColumns.${a.skillColumn}`),
            t(`equipment.armorRows.${a.armorRow}`),
          ],
        }));
      case "shield":
        return c.shields.map((s) => ({
          code: s.code,
          name: s.name,
          cells: [
            t(`equipment.shieldKinds.${s.shieldKind}`),
            String(s.acMelee),
            String(s.acMissile),
          ],
        }));
    }
  })();
  const q = fold(query);
  return q
    ? rows.filter((r) => fold(localized(lang, r.name)).includes(q))
    : rows;
}

/** What the player sets on the copy (ADR 0035 item 10, "Votre exemplaire"). */
export interface CopyDraft {
  /** The name as shown; sent only when the player changed it from the row's. */
  name: string;
  /** The "+N": a weapon's to hit and damage, an ammunition's bonus; none for armour and shields. */
  magic: number;
  /** Ammunition only. */
  count: string;
  equip: boolean;
}

/** Whether a category's copy takes a "+N" here (the armour and shield "+N" is a power, ADR 0033 step 17). */
export function hasMagic(category: PickedCategory): boolean {
  return category === "weapon" || category === "ammunition";
}

/** Whether a category's copy can be equipped at once (ammunition is carried only, ADR 0033 step 6). */
export function canEquip(category: PickedCategory): boolean {
  return category !== "ammunition";
}

/**
 * The POST /api/items body that picks `row`: its code; the name in the UI language only when the
 * player changed it (the row's other languages stay); the magic and count of the category; equip.
 * null when the count typed is not a number (the server validates the rest).
 */
export function pickBody(
  row: CatalogueRow,
  category: PickedCategory,
  copy: CopyDraft,
  lang: string,
): ItemBody | null {
  const body: ItemBody = { catalogueCode: row.code };
  const name = copy.name.trim();
  if (name && name !== localized(lang, row.name))
    body.name = { [editLang(lang)]: name };
  if (category === "weapon" && copy.magic !== 0) {
    body.magicToHit = copy.magic;
    body.magicDamage = copy.magic;
  }
  if (category === "ammunition") {
    if (copy.magic !== 0) body.magicalBonus = copy.magic;
    const count = Number(copy.count);
    if (!Number.isInteger(count) || count < 1 || copy.count.trim() === "")
      return null;
    body.count = count;
  }
  if (canEquip(category) && copy.equip) body.equip = true;
  return body;
}
