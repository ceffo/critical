import { create } from "zustand";

/** The editors of the Skills tab: the weapon proficiencies, the non-weapon ones (ADR 0037 steps 7
 * and 12) and a thief's or a bard's skill points (ADR 0030 steps 17 and 18). */
export type SkillsEditor = "weapon" | "nonWeapon" | "skillPoints";

interface SkillsEditorStore {
  /** The editor open in the Skills tab; null for its lists. */
  editor: SkillsEditor | null;
  /** Opens `editor`; the act panel shows the Skills tab when it is opened from elsewhere (the
   * level-up's grants and the card's "to spend" reminder, ADR 0037 step 15). */
  open: (editor: SkillsEditor) => void;
  close: () => void;
}

/** Which Skills tab editor is open: UI state only, shared so that the character card can open one. */
export const useSkillsEditor = create<SkillsEditorStore>((set) => ({
  editor: null,
  open: (editor) => set({ editor }),
  close: () => set({ editor: null }),
}));
