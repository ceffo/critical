import { create } from "zustand";
import type { BuffEffect, RoundAdvanceResponse } from "../types/buffs";
import * as api from "../api/buffs";
import * as roundApi from "../api/round";

interface BuffsStore {
  buffs: BuffEffect[];
  round: number;
  fetchBuffs: () => Promise<void>;
  fetchRound: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  activateBuff: (buff: Partial<BuffEffect>) => Promise<void>;
  dismissBuff: (id: string) => Promise<void>;
  triggerBuff: (id: string) => Promise<void>;
  advanceRound: () => Promise<RoundAdvanceResponse>;
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  round: 0,
  fetchBuffs: async () => {
    const buffs = await api.fetchActiveBuffs();
    set({ buffs: buffs ?? [] });
  },
  fetchRound: async () => {
    const { round } = await roundApi.fetchRound();
    set({ round });
  },
  activateBuff: async (buff) => {
    set({ buffs: (await api.activateBuff(buff)) ?? [] });
  },
  dismissBuff: async (id) => {
    set({ buffs: (await api.dismissBuff(id)) ?? [] });
  },
  triggerBuff: async (id) => {
    set({ buffs: (await api.triggerBuff(id)) ?? [] });
  },
  advanceRound: async () => {
    const res = await roundApi.advanceRound();
    set({ round: res.roundNumber, buffs: res.activeBuffs ?? [] });
    return res;
  },
}));
