import { create } from "zustand";
import { fetchSkills, setSkillPoints } from "@/api/skills";
import { ApiError } from "@/api/http";
import type { Skills } from "@/types/skills";
import { useTableStore } from "./table";

interface SkillsStore {
  skills: Skills | null;
  fetchSkills: () => Promise<void>;
  /** A points write in flight. */
  savingPoints: boolean;
  /** Sets a thief's points per skill (ADR 0030 step 17). A move on the action stack: the table is
   * refreshed with the character (its budget warning). Null on success, or the ApiError naming
   * the rejected field; other failures go to the table's error notice. */
  savePoints: (points: Record<string, number>) => Promise<ApiError | null>;
}

/** The Compétences tab: proficiencies, bard and thief skills with their targets and last checks. */
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
  savingPoints: false,
  savePoints: async (points) => {
    set({ savingPoints: true });
    try {
      set({ skills: await setSkillPoints(points) });
      await useTableStore.getState().refresh({ character: true });
      return null;
    } catch (e) {
      if (e instanceof ApiError && e.field) return e;
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      });
      return null;
    } finally {
      set({ savingPoints: false });
    }
  },
}));
