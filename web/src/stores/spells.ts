import { create } from "zustand";
import type { PreparationInfo, SpellSlot } from "../types/spells";
import * as api from "../api/spells";

interface SpellStore {
  slots: SpellSlot[];
  info: PreparationInfo;
  loading: boolean;
  fetchSlots: () => Promise<void>;
  fetchInfo: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  castSpell: (index: number) => Promise<void>;
  resetSlots: () => Promise<void>;
  prepare: (spells: string[]) => Promise<void>;
}

export const useSpellStore = create<SpellStore>((set) => ({
  slots: [],
  info: { spellbook: [], limits: [] },
  loading: false,
  fetchSlots: async () => {
    set({ loading: true });
    try {
      set({ slots: (await api.fetchSpellSlots()) ?? [] });
    } finally {
      set({ loading: false });
    }
  },
  fetchInfo: async () => {
    set({ info: await api.fetchPreparationInfo() });
  },
  castSpell: async (index) => {
    set({ slots: (await api.castSpell(index)) ?? [] });
  },
  resetSlots: async () => {
    set({ slots: (await api.resetSpellSlots()) ?? [] });
  },
  prepare: async (spells) => {
    set({ slots: (await api.prepareSpells(spells)) ?? [] });
  },
}));
