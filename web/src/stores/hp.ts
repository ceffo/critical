import { create } from "zustand";
import type { HpEvent, HpState } from "@/types/hp";
import { getHpState, appendHpEvent, undoLastEvent } from "@/api/hp";

interface HpStore extends HpState {
  fetchHp: () => Promise<void>;
  appendEvent: (delta: number, source: string) => Promise<void>;
  undoLastEvent: () => Promise<void>;
}

export const useHpStore = create<HpStore>((set) => ({
  currentHp: 0,
  maxHp: 0,
  events: [] as HpEvent[],
  fetchHp: async () => {
    const state = await getHpState();
    set({
      ...state,
      events: state.events || [],
    });
  },
  appendEvent: async (delta: number, source: string) => {
    const state = await appendHpEvent(delta, source);
    set({
      ...state,
      events: state.events || [],
    });
  },
  undoLastEvent: async () => {
    const state = await undoLastEvent();
    set({
      ...state,
      events: state.events || [],
    });
  },
}));
