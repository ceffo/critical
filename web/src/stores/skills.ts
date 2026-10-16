import { create } from "zustand";
import { fetchSkills } from "@/api/skills";
import type { SkillKind, Skills } from "@/types/skills";
import { useTableStore } from "./table";

interface SkillsStore {
  skills: Skills | null;
  /** The DM adjustment of the next check of each section (UI state: not persisted, sent with the roll, reset after it). */
  pending: Record<SkillKind, number>;
  setPending: (kind: SkillKind, value: number) => void;
  fetchSkills: () => Promise<void>;
}

/** The Compétences tab: proficiencies and bard skills with their targets and last checks. */
export const useSkillsStore = create<SkillsStore>((set) => ({
  skills: null,
  pending: { proficiency: 0, bardSkill: 0 },
  setPending: (kind, value) =>
    set((s) => ({ pending: { ...s.pending, [kind]: value } })),
  fetchSkills: async () => {
    try {
      set({ skills: await fetchSkills() });
    } catch (e) {
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      });
    }
  },
}));
