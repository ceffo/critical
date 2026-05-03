import { create } from "zustand";
import type { SpellSlot } from "../types/spells";
import * as api from "../api/spells";

interface SpellStore {
  slots: SpellSlot[];
  loading: boolean;
  error: string | null;

  fetchSlots: () => Promise<void>;
  castSpell: (index: number) => Promise<void>;
  resetSlots: () => Promise<void>;
}

export const useSpellStore = create<SpellStore>((set) => ({
  slots: [],
  loading: false,
  error: null,

  fetchSlots: async () => {
    set({ loading: true, error: null });
    try {
      const slots = await api.fetchSpellSlots();
      set({ slots, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  castSpell: async (index: number) => {
    set({ loading: true, error: null });
    try {
      const slots = await api.castSpell(index);
      set({ slots, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  resetSlots: async () => {
    set({ loading: true, error: null });
    try {
      const slots = await api.resetSpellSlots();
      set({ slots, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },
}));
