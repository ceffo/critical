import { create } from "zustand";
import type { RollResult } from "../types/combat";
import { resolveAttack } from "../api/combat";

interface CombatStore {
  lastResult: RollResult | null;
  isRolling: boolean;
  error: string | null;
  rollAttack: (weaponId: string, targetAc: number) => Promise<void>;
  clearResult: () => void;
}

export const useCombatStore = create<CombatStore>((set) => ({
  lastResult: null,
  isRolling: false,
  error: null,
  rollAttack: async (weaponId, targetAc) => {
    set({ isRolling: true, error: null });
    try {
      const result = await resolveAttack(weaponId, targetAc);
      set({ lastResult: result, isRolling: false });
    } catch (err) {
      set({ error: (err as Error).message, isRolling: false });
    }
  },
  clearResult: () => set({ lastResult: null, error: null }),
}));
