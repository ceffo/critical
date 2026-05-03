import { create } from 'zustand';
import type { HpEvent, HpState } from '@/types/hp';
import { getHpState, appendHpEvent, undoLastEvent } from '@/api/hp';

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
    set(state);
  },
  appendEvent: async (delta: number, source: string) => {
    await appendHpEvent(delta, source);
    const state = await getHpState();
    set(state);
  },
  undoLastEvent: async () => {
    const state = await undoLastEvent();
    set(state);
  },
}));
