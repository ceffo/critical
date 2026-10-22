import { create } from "zustand";

/** The editors of the Skills tab: the weapon proficiencies, the non-weapon ones (ADR 0037 steps 7
 * and 12) and a thief's or a bard's skill points (ADR 0030 steps 17 and 18). */
export type SkillsEditor = "weapon" | "nonWeapon" | "skillPoints";

interface SkillsEditorStore {
  /** The editor open in the Skills tab; null for its lists. */
  editor: SkillsEditor | null;
  /** The skill set the points editor is for; null lets the tab pick the first set with points
   * left to spend (the level-up's grants cannot name one). */
  set: string | null;
  /** Opens `editor` (for the skill set `set`, the points editor's); the act panel shows the Skills
   * tab when it is opened from elsewhere (the level-up's grants and the card's "to spend"
   * reminder, ADR 0037 step 15). */
  open: (editor: SkillsEditor, set?: string) => void;
  close: () => void;
}

/** Which Skills tab editor is open: UI state only, shared so that the character card can open one. */
export const useSkillsEditor = create<SkillsEditorStore>((set) => ({
  editor: null,
  set: null,
  open: (editor, skillSet) => set({ editor, set: skillSet ?? null }),
  close: () => set({ editor: null, set: null }),
}));
