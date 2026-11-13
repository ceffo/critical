import type { Category, EquipmentRules, MagicKind } from "@/types/items";
import { CATEGORIES, MAGIC_KINDS } from "@/types/items";

/**
 * GET /api/equipment/rules as the server serves it today (itemsvc.QuantityRules, ADR 0049 step 2):
 * ammunition, gear, a potion and a scroll have a quantity. A test fixture of the wire, never read by
 * production code.
 */
export const equipmentRules: EquipmentRules = {
  maxQuantity: 999,
  categories: CATEGORIES.map((category: Category) => ({
    category,
    hasQuantity: category === "ammunition" || category === "gear",
    magicKinds:
      category === "magic"
        ? MAGIC_KINDS.map((kind: MagicKind) => ({
            kind,
            hasQuantity: kind === "potion" || kind === "scroll",
          }))
        : [],
  })),
};
