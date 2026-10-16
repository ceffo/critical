import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { useItemsStore } from "@/stores/items";
import type { LocalizedText } from "@/types/localized";

/**
 * Names an item or a weapon recorded by id in a result or a log payload (ADR 0023 step 12): its
 * current name in the player's language while the inventory has it, else the name recorded with the
 * payload (an item since removed, an entry older than the id).
 */
export interface InventoryNamer {
  item: (id: string | undefined, recorded: string) => string;
  weapon: (id: string | undefined, recorded: string) => string;
}

/** What the namer reads of an item or a weapon. */
interface Named {
  id: string;
  name: LocalizedText;
}

/** The namer over the current item and weapon lists, in the UI language `lang`. */
export function inventoryNamer(
  items: readonly Named[],
  weapons: readonly Named[],
  lang: string,
): InventoryNamer {
  const named =
    (list: readonly Named[]) => (id: string | undefined, recorded: string) => {
      const found = id ? list.find((x) => x.id === id) : undefined;
      return (found && localized(lang, found.name)) || recorded;
    };
  return { item: named(items), weapon: named(weapons) };
}

/** Names as recorded in the payloads (tests, no inventory). */
export const recordedNames: InventoryNamer = {
  item: (_, recorded) => recorded,
  weapon: (_, recorded) => recorded,
};

/** The namer for the current language and lists; re-renders when either changes. */
export function useInventoryName(): InventoryNamer {
  const { i18n } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const weapons = useItemsStore((s) => s.weapons);
  return inventoryNamer(items, weapons, i18n.language);
}
