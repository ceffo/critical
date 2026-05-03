import { create } from 'zustand';
import type { BuffEffect } from '../types/buffs';
import * as api from '../api/buffs';
import * as roundApi from '../api/round';

interface BuffsStore {
  buffs: BuffEffect[];
  round: number;
  loading: boolean;
  error: string | null;
  fetchBuffs: () => Promise<void>;
  fetchRound: () => Promise<void>;
  activateBuff: (buff: Partial<BuffEffect>) => Promise<void>;
  dismissBuff: (id: string) => Promise<void>;
  triggerBuff: (id: string) => Promise<void>;
  advanceRound: () => Promise<BuffEffect[]>; // returns expired buffs
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  round: 0,
  loading: false,
  error: null,
  fetchBuffs: async () => {
    set({ loading: true });
    try {
      const buffs = await api.fetchActiveBuffs();
      set({ buffs, loading: false });
    } catch (e: any) {
      set({ error: e.message, loading: false });
    }
  },
  fetchRound: async () => {
    try {
      const { roundNumber } = await roundApi.fetchRound();
      set({ round: roundNumber });
    } catch (e: any) {
      set({ error: e.message });
    }
  },
  activateBuff: async (buff) => {
    try {
      const buffs = await api.activateBuff(buff);
      set({ buffs });
    } catch (e: any) {
      set({ error: e.message });
    }
  },
  dismissBuff: async (id) => {
    try {
      const buffs = await api.dismissBuff(id);
      set({ buffs });
    } catch (e: any) {
      set({ error: e.message });
    }
  },
  triggerBuff: async (id) => {
    try {
      const buffs = await api.triggerBuff(id);
      set({ buffs });
    } catch (e: any) {
      set({ error: e.message });
    }
  },
  advanceRound: async () => {
    try {
      const res = await roundApi.advanceRound();
      set({ round: res.roundNumber, buffs: res.activeBuffs });
      return res.expiredBuffs;
    } catch (e: any) {
      set({ error: e.message });
      return [];
    }
  },
}));
