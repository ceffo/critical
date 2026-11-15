import { create } from "zustand";

/** The editors of the Skills activity: the weapon proficiencies, the non-weapon ones (ADR 0037
 * steps 7 and 12) and a thief's or a bard's skill points (ADR 0030 steps 17 and 18). */
export type SkillsEditor = "weapon" | "nonWeapon" | "skillPoints";

interface SkillsEditorStore {
  /** The editor asked for from elsewhere (the level-up's links); the Skills activity takes the
   * request, scrolls to that editor and clears it. Null when none is pending. */
  editor: SkillsEditor | null;
  /** The skill set the points editor is for; null lets the activity pick the first set with
   * points left to spend (the level-up's grants cannot name one). */
  set: string | null;
  /** Asks the Skills activity to show `editor` (for the skill set `set`, the points editor's);
   * the shell focuses the activity (ADR 0037 step 15's links). */
  open: (editor: SkillsEditor, set?: string) => void;
  close: () => void;
}

/** Which Skills editor is asked for: UI state only, shared so that the character card can ask. */
export const useSkillsEditor = create<SkillsEditorStore>((set) => ({
  editor: null,
  set: null,
  open: (editor, skillSet) => set({ editor, set: skillSet ?? null }),
  close: () => set({ editor: null, set: null }),
}));
