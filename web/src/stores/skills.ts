import { create } from "zustand";
import { fetchSkills } from "@/api/skills";
import type { Skills } from "@/types/skills";
import { useTableStore } from "./table";

interface SkillsStore {
  skills: Skills | null;
  fetchSkills: () => Promise<void>;
}

/** The Compétences tab: proficiencies and bard skills with their targets and last checks. */
export const useSkillsStore = create<SkillsStore>((set) => ({
  skills: null,
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
