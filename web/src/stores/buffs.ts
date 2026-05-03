import { create } from "zustand";
import type { BuffEffect } from "../types/buffs";
import * as api from "../api/buffs";
import * as roundApi from "../api/round";

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
      set({ buffs: buffs || [], loading: false });
    } catch (e: unknown) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  fetchRound: async () => {
    try {
      const { roundNumber } = await roundApi.fetchRound();
      set({ round: roundNumber });
    } catch (e: unknown) {
      set({ error: (e as Error).message });
    }
  },
  activateBuff: async (buff) => {
    try {
      const buffs = await api.activateBuff(buff);
      set({ buffs: buffs || [] });
    } catch (e: unknown) {
      set({ error: (e as Error).message });
    }
  },
  dismissBuff: async (id) => {
    try {
      const buffs = await api.dismissBuff(id);
      set({ buffs: buffs || [] });
    } catch (e: unknown) {
      set({ error: (e as Error).message });
    }
  },
  triggerBuff: async (id) => {
    try {
      const buffs = await api.triggerBuff(id);
      set({ buffs: buffs || [] });
    } catch (e: unknown) {
      set({ error: (e as Error).message });
    }
  },
  advanceRound: async () => {
    try {
      const res = await roundApi.advanceRound();
      set({
        round: res.roundNumber,
        buffs: res.activeBuffs || [],
      });
      return res.expiredBuffs || [];
    } catch (e: unknown) {
      set({ error: (e as Error).message });
      return [];
    }
  },
}));
