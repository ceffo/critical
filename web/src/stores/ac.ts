import { create } from "zustand";
import type { ArmourClass } from "@/types/ac";
import { fetchAc } from "@/api/ac";

interface AcStore {
  /** Null until first read. */
  ac: ArmourClass | null;
  fetchAc: () => Promise<void>;
  set: (ac: ArmourClass) => void;
}

/** The character's AC as the server derives it: re-read with the rest of the table after every action. */
export const useAcStore = create<AcStore>((set) => ({
  ac: null,
  fetchAc: async () => set({ ac: await fetchAc() }),
  set: (ac) => set({ ac }),
}));
