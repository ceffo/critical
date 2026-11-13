import type { TFunction } from "i18next";
import i18n from "@/i18n";
import { localized } from "@/i18n/localized";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import type { EquipmentCatalogue } from "@/types/items";

/**
 * A weapon pick's name (ADR 0037 step 21.6): the catalogue weapon of `code` named from the
 * equipment catalogue (its language map, in the UI language), else the weapon type's word when the
 * code is also a type ("dagger"), else the code itself. No i18n key per weapon.
 */
export function weaponName(
  t: TFunction,
  code: string,
  catalogue: EquipmentCatalogue | null = useEquipmentCatalogueStore.getState()
    .catalogue,
  lang: string = i18n.language,
): string {
  const row = catalogue?.weapons.find((w) => w.code === code);
  const name = row ? localized(lang, row.name) : "";
  return name || t(`weapons.types.${code}`, { defaultValue: code });
}
