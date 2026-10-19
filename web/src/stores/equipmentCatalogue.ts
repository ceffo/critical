import { create } from "zustand";
import type { EquipmentCatalogue } from "@/types/items";
import { fetchEquipmentCatalogue } from "@/api/items";

interface EquipmentCatalogueStore {
  /** null until read. */
  catalogue: EquipmentCatalogue | null;
  /** Reads the catalogue once: it is reference data, no write changes it (no state key). */
  load: () => Promise<void>;
}

/** The equipment rows Ajouter > Catalogue picks from (ADR 0033 step 24, ADR 0035 item 10). */
export const useEquipmentCatalogueStore = create<EquipmentCatalogueStore>(
  (set, get) => ({
    catalogue: null,
    load: async () => {
      if (get().catalogue) return;
      set({ catalogue: await fetchEquipmentCatalogue() });
    },
  }),
);
