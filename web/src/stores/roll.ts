import { create } from "zustand";
import type { AttackRequest, RollResult, SaveCategory } from "@/types/combat";
import type { CastResult } from "@/types/spells";
import { resolveAttack } from "@/api/combat";
import { rollSavingThrow } from "@/api/roll";
import { useTableStore } from "./table";

/** The last roll and what was asked for, so the result card can show its context. */
export type LastRoll =
  | { kind: "attack"; result: RollResult }
  | { kind: "save"; category: SaveCategory; result: RollResult }
  | { kind: "cast"; result: CastResult };

interface RollStore {
  last: LastRoll | null;
  rolling: boolean;
  /** Resolves true when the attack was rolled (and logged). */
  rollAttack: (req: AttackRequest) => Promise<boolean>;
  rollSave: (category: SaveCategory) => Promise<void>;
}

export const useRollStore = create<RollStore>((set) => {
  async function roll(fn: () => Promise<LastRoll>, refresh: boolean): Promise<boolean> {
    set({ rolling: true });
    const table = useTableStore.getState();
    table.clearError();
    let ok = true;
    try {
      set({ last: await fn() });
    } catch (e) {
      ok = false;
      useTableStore.setState({ error: e instanceof Error ? e.message : String(e) });
    } finally {
      set({ rolling: false });
    }
    // An attack is logged and may spend ammo: show the new log entry and counts.
    if (refresh) await table.refresh();
    return ok;
  }
  return {
    last: null,
    rolling: false,
    rollAttack: (req) =>
      roll(async () => ({ kind: "attack", result: await resolveAttack(req) }), true),
    rollSave: async (category) => {
      await roll(
        async () => ({ kind: "save", category, result: await rollSavingThrow(category) }),
        false,
      );
    },
  };
});
