import { create } from "zustand";
import type {
  BuffEdit,
  BuffEffect,
  RoundAdvanceResponse,
  Strength,
} from "../types/buffs";
import * as api from "../api/buffs";
import * as roundApi from "../api/round";

interface BuffsStore {
  buffs: BuffEffect[];
  round: number;
  /** Sheet STR and STR with buffs (null until loaded). */
  strength: Strength | null;
  fetchBuffs: () => Promise<void>;
  fetchRound: () => Promise<void>;
  fetchStrength: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  activateBuff: (buff: Partial<BuffEffect>) => Promise<void>;
  editBuff: (id: string, edit: BuffEdit) => Promise<void>;
  dismissBuff: (id: string) => Promise<void>;
  triggerBuff: (id: string) => Promise<void>;
  advanceRound: () => Promise<RoundAdvanceResponse>;
  resetRound: () => Promise<RoundAdvanceResponse>;
  setRound: (round: number) => Promise<RoundAdvanceResponse>;
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  round: 0,
  strength: null,
  fetchBuffs: async () => {
    const buffs = await api.fetchActiveBuffs();
    set({ buffs: buffs ?? [] });
  },
  fetchRound: async () => {
    const { round } = await roundApi.fetchRound();
    set({ round });
  },
  fetchStrength: async () => {
    set({ strength: await api.fetchStrength() });
  },
  activateBuff: async (buff) => {
    set({ buffs: (await api.activateBuff(buff)) ?? [] });
  },
  editBuff: async (id, edit) => {
    set({ buffs: (await api.editBuff(id, edit)) ?? [] });
  },
  dismissBuff: async (id) => {
    set({ buffs: (await api.dismissBuff(id)) ?? [] });
  },
  triggerBuff: async (id) => {
    set({ buffs: (await api.triggerBuff(id)) ?? [] });
  },
  advanceRound: async () => {
    const res = await roundApi.advanceRound();
    set({ round: res.roundNumber });
    return res;
  },
  resetRound: async () => {
    const res = await roundApi.resetRound();
    set({ round: res.roundNumber });
    return res;
  },
  setRound: async (round) => {
    const res = await roundApi.setRound(round);
    set({ round: res.roundNumber });
    return res;
  },
}));
