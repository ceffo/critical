import type { SaveNumbers } from "@/types/character";
import type { SaveCategory } from "@/types/combat";

/** The field of the server's per-category save numbers (targets, race bonuses) of each category. */
export const SAVE_FIELD: Record<SaveCategory, keyof SaveNumbers> = {
  para_poison: "paraPoison",
  rod_staff_wand: "rodStaffWand",
  petri_poly: "petriPoly",
  breath: "breath",
  spell: "spell",
};
