import { create } from 'zustand';
import type { Buff } from '@/types/buffs';
import { getBuffs } from '@/api/buffs';

interface BuffsStore {
  buffs: Buff[];
  fetchBuffs: () => Promise<void>;
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  fetchBuffs: async () => {
    const buffs = await getBuffs();
    set({ buffs });
  },
}));
