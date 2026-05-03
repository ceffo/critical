import { create } from 'zustand';
import type { RollResult } from '@/types/combat';
import { rollAttack } from '@/api/combat';

interface CombatStore {
  lastRoll: RollResult | null;
  roll: (targetAC: number) => Promise<void>;
}

export const useCombatStore = create<CombatStore>((set) => ({
  lastRoll: null,
  roll: async (targetAC: number) => {
    const result = await rollAttack(targetAC);
    set({ lastRoll: result });
  },
}));
