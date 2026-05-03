import { create } from 'zustand';
import type { Spell } from '@/types/spells';
import { searchSpells } from '@/api/spells_db';

interface SpellsDbStore {
  results: Spell[];
  loading: boolean;
  error: string | null;
  search: (q: string, cls?: string) => Promise<void>;
}

export const useSpellsDbStore = create<SpellsDbStore>((set) => ({
  results: [],
  loading: false,
  error: null,
  search: async (q, cls) => {
    set({ loading: true });
    try {
      const results = await searchSpells(q, cls);
      set({ results, loading: false, error: null });
    } catch (e: unknown) {
      set({ error: (e as Error).message, loading: false });
    }
  },
}));
