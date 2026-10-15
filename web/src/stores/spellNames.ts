import { create } from "zustand";
import { getSpellNames } from "@/api/spells_db";
import type { SpellNames } from "@/types/spells";

/**
 * Every catalogue spell's names by id (GET /api/spells/names), loaded once at start: the UI names
 * a spell it knows only by id (spellbook, slots, journal, buffs) in the player's language.
 */
interface SpellNamesStore {
  names: SpellNames;
  load: () => Promise<void>;
}

export const useSpellNamesStore = create<SpellNamesStore>((set) => ({
  names: {},
  load: async () => {
    let list;
    try {
      list = await getSpellNames();
    } catch {
      return; // names stay English (the callers' own), the rest of the UI works
    }
    set({ names: Object.fromEntries(list.map((n) => [n.id, n])) });
  },
}));
