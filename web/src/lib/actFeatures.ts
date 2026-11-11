import { useCharacterStore } from "@/stores/character";
import { useItemsStore } from "@/stores/items";
import { useSpellStore } from "@/stores/spells";
import type { Item } from "@/types/items";

/**
 * What a character has any use for in the Act activity and the rail (ADR 0022 steps 2.4 and 2.5), read from
 * what the server serves and never from a class: the spell slots (the sheet's, summed over every
 * class entry, so a multi-class character has spells when any entry does), the known-powers
 * block, and the per-day items the long rest refills.
 */
export interface ActFeatures {
  hasSpells: boolean;
  hasPowers: boolean;
  /** The Spells tab: spells, or a per-day item the Long rest button there refills. */
  showSpellsTab: boolean;
}

export function actFeatures(
  slots: number,
  knownPowers: boolean,
  items: readonly Pick<Item, "behaviour">[],
): ActFeatures {
  const hasSpells = slots > 0;
  return {
    hasSpells,
    hasPowers: knownPowers,
    showSpellsTab: hasSpells || items.some((i) => i.behaviour === "daily"),
  };
}

/** The rail's second activity is named by its content (ADR 0036): the key under `shell.activity`. */
export function lookupLabelKey(f: ActFeatures): string {
  if (f.hasSpells && f.hasPowers) return "spellsAndPowers";
  if (f.hasPowers) return "powers";
  if (f.hasSpells) return "spells";
  return "reference";
}

export function useActFeatures(): ActFeatures {
  // The sheet's per-day counts follow the character at once; the slot list adds extra slots.
  const slots = useSpellStore((s) => s.slots.length);
  const perDay = useCharacterStore(
    (s) => s.character?.derived.spellSlots.length ?? 0,
  );
  const knownPowers = useCharacterStore(
    (s) => s.character?.derived.knownPowers != null,
  );
  const items = useItemsStore((s) => s.items);
  return actFeatures(slots + perDay, knownPowers, items);
}
