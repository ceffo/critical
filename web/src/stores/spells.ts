import { create } from "zustand";
import type { PreparationInfo, SpellSlot } from "../types/spells";
import * as api from "../api/spells";
import { useRollStore } from "./roll";

interface SpellStore {
  slots: SpellSlot[];
  info: PreparationInfo;
  loading: boolean;
  fetchSlots: () => Promise<void>;
  fetchInfo: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  /** Cast from slot `index`; the result goes to the result card. */
  castSpell: (index: number, opts?: api.CastOptions) => Promise<void>;
  /** Apply a spell a party member cast on the Bard; the result goes to the result card. */
  receiveSpell: (spellId: string, casterLevel: number, mode?: string) => Promise<void>;
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
  castSpell: async (index, opts) => {
    const result = await api.castSpell(index, opts);
    set({ slots: result.slots ?? [] });
    useRollStore.setState({ last: { kind: "cast", result } });
  },
  receiveSpell: async (spellId, casterLevel, mode) => {
    const result = await api.receiveSpell(spellId, casterLevel, mode);
    useRollStore.setState({ last: { kind: "cast", result } });
  },
  resetSlots: async () => {
    set({ slots: (await api.resetSpellSlots()) ?? [] });
  },
  prepare: async (spells) => {
    set({ slots: (await api.prepareSpells(spells)) ?? [] });
  },
}));
