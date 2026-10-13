import { create } from "zustand";
import type { Spell } from "@/types/spells";
import { searchSpells, type SpellFilters } from "@/api/spells_db";

interface SpellsDbStore {
  results: Spell[];
  loading: boolean;
  error: string | null;
  search: (f: SpellFilters) => Promise<void>;
}

export const useSpellsDbStore = create<SpellsDbStore>((set) => ({
  results: [],
  loading: false,
  error: null,
  search: async (f) => {
    set({ loading: true });
    try {
      const results = await searchSpells(f);
      set({ results, loading: false, error: null });
    } catch (e: unknown) {
      set({ error: (e as Error).message, loading: false });
    }
  },
}));
