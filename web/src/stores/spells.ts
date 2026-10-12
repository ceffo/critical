import { create } from "zustand";
import type { SpellSlot } from "../types/spells";
import * as api from "../api/spells";

interface SpellStore {
  slots: SpellSlot[];
  loading: boolean;
  fetchSlots: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  castSpell: (index: number) => Promise<void>;
  resetSlots: () => Promise<void>;
}

export const useSpellStore = create<SpellStore>((set) => ({
  slots: [],
  loading: false,
  fetchSlots: async () => {
    set({ loading: true });
    try {
      set({ slots: (await api.fetchSpellSlots()) ?? [] });
    } finally {
      set({ loading: false });
    }
  },
  castSpell: async (index) => {
    set({ slots: (await api.castSpell(index)) ?? [] });
  },
  resetSlots: async () => {
    set({ slots: (await api.resetSpellSlots()) ?? [] });
  },
}));
