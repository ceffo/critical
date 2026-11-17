import { create } from "zustand";

/** The ability a proficiency row is about, while the pointer or the keyboard is on it: the
 * character card's score for it is highlighted, so the score a check rolls under stays in sight.
 * UI state only. */
interface AbilityFocusStore {
  ability: string | null;
  focus: (ability: string | undefined) => void;
  clear: () => void;
}

export const useAbilityFocus = create<AbilityFocusStore>((set) => ({
  ability: null,
  focus: (ability) => set({ ability: ability ?? null }),
  clear: () => set({ ability: null }),
}));
