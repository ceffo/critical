import { create } from "zustand";
import type { EquipmentRules } from "@/types/items";
import { fetchEquipmentRules } from "@/api/items";

interface EquipmentRulesStore {
  /** null until read. */
  rules: EquipmentRules | null;
  /** Reads the rules once: reference data, no write changes it (no state key). */
  load: () => Promise<void>;
}

/** The server's per-category rules the item form shows its fields by (ADR 0049 step 2). */
export const useEquipmentRulesStore = create<EquipmentRulesStore>(
  (set, get) => ({
    rules: null,
    load: async () => {
      if (get().rules) return;
      set({ rules: await fetchEquipmentRules() });
    },
  }),
);
