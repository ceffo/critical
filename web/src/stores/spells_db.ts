import { create } from "zustand";
import type { Spell } from "@/types/spells";
import { searchSpells, type SpellFilters } from "@/api/spells_db";

interface SpellsDbStore {
  results: Spell[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  error: string | null;
  search: (f: SpellFilters, page?: number) => Promise<void>;
}

export const useSpellsDbStore = create<SpellsDbStore>((set) => ({
  results: [],
  total: 0,
  page: 1,
  pageSize: 50,
  loading: false,
  error: null,
  search: async (f, page = 1) => {
    set({ loading: true });
    try {
      const r = await searchSpells(f, page);
      set({
        results: r.items,
        total: r.total,
        page: r.page,
        pageSize: r.page_size,
        loading: false,
        error: null,
      });
    } catch (e: unknown) {
      set({ error: (e as Error).message, loading: false });
    }
  },
}));
