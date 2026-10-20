import { create } from "zustand";
import { fetchSkills, setSkillPoints } from "@/api/skills";
import { ApiError } from "@/api/http";
import { setProficiencies } from "@/api/proficiencies";
import type { PicksInput } from "@/types/character";
import type { Skills } from "@/types/skills";
import { useTableStore } from "./table";

interface SkillsStore {
  skills: Skills | null;
  /** Throws on failure: read it through the action module's refresh. */
  fetchSkills: () => Promise<void>;
  /** Sets a thief's points per skill (ADR 0030 step 17), a move on the action stack run by the
   * action module. Null on success, or the ApiError naming the rejected field; other failures go
   * to the action module's error notice. */
  savePoints: (points: Record<string, number>) => Promise<ApiError | null>;
  /** Saves the non-weapon picks (ADR 0037 step 12), a move on the action stack; the skills and
   * the sheet are read again as the server names them. Null on success, or the ApiError naming
   * the refused field (nonweapon.<id>, or nonweapon for an over-budget save). */
  saveProficiencies: (picks: PicksInput) => Promise<ApiError | null>;
}

/** The Compétences tab: proficiencies, bard and thief skills with their targets and last checks. */
export const useSkillsStore = create<SkillsStore>((set) => ({
  skills: null,
  fetchSkills: async () => set({ skills: await fetchSkills() }),
  // The answer is the skills: they are not read again.
  savePoints: (points) =>
    useTableStore
      .getState()
      .submit(async () => set({ skills: await setSkillPoints(points) }), {
        fresh: ["skills"],
      }),
  saveProficiencies: (picks) =>
    useTableStore.getState().submit(() => setProficiencies(picks)),
}));
