import { create } from "zustand";
import type { RollResult, SaveCategory } from "@/types/combat";
import { resolveAttack } from "@/api/combat";
import { rollSavingThrow } from "@/api/roll";
import { useTableStore } from "./table";

/** The last roll and what was asked for, so the result card can show its context. */
export type LastRoll =
  | { kind: "attack"; weaponId: string; targetAc: number; result: RollResult }
  | { kind: "save"; category: SaveCategory; result: RollResult };

interface RollStore {
  last: LastRoll | null;
  rolling: boolean;
  rollAttack: (weaponId: string, targetAc: number) => Promise<void>;
  rollSave: (category: SaveCategory) => Promise<void>;
}

export const useRollStore = create<RollStore>((set) => {
  async function roll(fn: () => Promise<LastRoll>) {
    set({ rolling: true });
    useTableStore.getState().clearError();
    try {
      set({ last: await fn() });
    } catch (e) {
      useTableStore.setState({ error: e instanceof Error ? e.message : String(e) });
    } finally {
      set({ rolling: false });
    }
  }
  return {
    last: null,
    rolling: false,
    rollAttack: (weaponId, targetAc) =>
      roll(async () => ({
        kind: "attack",
        weaponId,
        targetAc,
        result: await resolveAttack(weaponId, targetAc),
      })),
    rollSave: (category) =>
      roll(async () => ({
        kind: "save",
        category,
        result: await rollSavingThrow(category),
      })),
  };
});
