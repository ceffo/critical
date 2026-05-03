import { create } from 'zustand';
import type { SpellSlot } from '@/types/spells';
import { getSpellSlots } from '@/api/spells';

interface SpellsStore {
  slots: SpellSlot[];
  fetchSlots: () => Promise<void>;
}

export const useSpellsStore = create<SpellsStore>((set) => ({
  slots: [],
  fetchSlots: async () => {
    const slots = await getSpellSlots();
    set({ slots });
  },
}));
