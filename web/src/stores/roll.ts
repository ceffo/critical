import { create } from "zustand";
import type { AttackRequest, RollResult, SaveCategory } from "@/types/combat";
import type { CastResult } from "@/types/spells";
import { resolveAttack } from "@/api/combat";
import { adjustAttack, adjustSave, rollSavingThrow } from "@/api/roll";
import { postUseItem } from "@/api/items";
import type { UseResult } from "@/types/items";
import { useTableStore } from "./table";

/** The last roll and what was asked for, so the result card can show its context. */
export type LastRoll =
  | { kind: "attack"; result: RollResult }
  | { kind: "save"; category: SaveCategory; result: RollResult }
  | { kind: "cast"; result: CastResult }
  | { kind: "item"; result: UseResult };

interface RollStore {
  last: LastRoll | null;
  rolling: boolean;
  /** Resolves true when the attack was rolled (and logged). */
  rollAttack: (req: AttackRequest) => Promise<boolean>;
  rollSave: (category: SaveCategory) => Promise<void>;
  /** Sets the adjustment of the save on the card; resolves true on success. */
  adjustLastSave: (adjustment: number, reason: string) => Promise<boolean>;
  /** Corrects the target AC of the attack on the card; resolves true on success. */
  adjustLastAttack: (targetAc: number) => Promise<boolean>;
  /** Uses one charge of an item (rolls its effect, logs it). Resolves true on success. */
  spendItem: (id: string) => Promise<boolean>;
}

export const useRollStore = create<RollStore>((set, get) => {
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
    spendItem: (id) => roll(async () => ({ kind: "item", result: await postUseItem(id) }), true),
    // A save is logged: refresh to show its entry.
    rollSave: async (category) => {
      await roll(
        async () => ({ kind: "save", category, result: await rollSavingThrow(category) }),
        true,
      );
    },
    adjustLastAttack: async (targetAc) => {
      const last = get().last;
      if (last?.kind !== "attack" || last.result.logEntryId === undefined) return false;
      const id = last.result.logEntryId;
      return roll(async () => ({ kind: "attack", result: await adjustAttack(id, targetAc) }), true);
    },
    adjustLastSave: async (adjustment, reason) => {
      const last = get().last;
      if (last?.kind !== "save" || last.result.logEntryId === undefined) return false;
      const id = last.result.logEntryId;
      return roll(
        async () => ({
          kind: "save",
          category: last.category,
          result: await adjustSave(id, adjustment, reason),
        }),
        true,
      );
    },
  };
});
