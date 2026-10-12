import { create } from "zustand";
import type { HpState } from "@/types/hp";
import { getHpState } from "@/api/hp";

interface HpStore extends HpState {
  loaded: boolean;
  fetchHp: () => Promise<void>;
  setHp: (state: HpState) => void;
}

/** Current/max HP as computed by the server. Mutations go through stores/table.ts. */
export const useHpStore = create<HpStore>((set) => ({
  currentHp: 0,
  maxHp: 0,
  events: [],
  loaded: false,
  fetchHp: async () => {
    const state = await getHpState();
    set({ ...state, events: state.events ?? [], loaded: true });
  },
  setHp: (state) => set({ ...state, events: state.events ?? [], loaded: true }),
}));
