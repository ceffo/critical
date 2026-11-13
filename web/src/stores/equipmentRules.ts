import { create } from "zustand";
import type { EquipmentRules } from "@/types/items";
import { fetchEquipmentRules } from "@/api/items";

interface EquipmentRulesStore {
  /** null until read. */
  rules: EquipmentRules | null;
  /** The last read failed: the form says so instead of silently hiding what the rules decide. */
  failed: boolean;
  /** Reads the rules once: reference data, no write changes it (no state key). A failure is kept in `failed` and retried by the next call. */
  load: () => Promise<void>;
}

/** The server's per-category rules the item form shows its fields by (ADR 0049 step 2). */
export const useEquipmentRulesStore = create<EquipmentRulesStore>(
  (set, get) => ({
    rules: null,
    failed: false,
    load: async () => {
      if (get().rules) return;
      try {
        set({ rules: await fetchEquipmentRules(), failed: false });
      } catch {
        set({ failed: true });
      }
    },
  }),
);
