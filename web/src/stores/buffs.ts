import { create } from "zustand";
import type {
  BuffEdit,
  BuffEffect,
  Initiative,
  RoundAdvanceResponse,
  SavedBuff,
  Strength,
} from "../types/buffs";
import * as api from "../api/buffs";
import * as roundApi from "../api/round";

interface BuffsStore {
  buffs: BuffEffect[];
  round: number;
  /** The round's initiative, from the server; null out of combat or until rolled. */
  initiative: Initiative | null;
  /** Saved buff templates, oldest first. */
  saved: SavedBuff[];
  /** Sheet STR and STR with buffs (null until loaded). */
  strength: Strength | null;
  fetchBuffs: () => Promise<void>;
  fetchSaved: () => Promise<void>;
  fetchRound: () => Promise<void>;
  fetchStrength: () => Promise<void>;
  /** Mutations throw on failure; call them through useTableStore.run to surface errors. */
  activateBuff: (
    buff: Omit<Partial<BuffEffect>, "startRound">,
  ) => Promise<void>;
  editBuff: (id: string, edit: BuffEdit) => Promise<void>;
  dismissBuff: (id: string) => Promise<void>;
  triggerBuff: (id: string) => Promise<void>;
  saveBuffTemplate: (saved: Omit<SavedBuff, "id">) => Promise<void>;
  deleteSavedBuff: (id: number) => Promise<void>;
  advanceRound: () => Promise<RoundAdvanceResponse>;
  resetRound: () => Promise<RoundAdvanceResponse>;
  setRound: (round: number) => Promise<RoundAdvanceResponse>;
  passTime: (hours: number) => Promise<RoundAdvanceResponse>;
  rerollInitiative: () => Promise<void>;
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  round: 0,
  initiative: null,
  saved: [],
  strength: null,
  fetchBuffs: async () => {
    const buffs = await api.fetchActiveBuffs();
    set({ buffs: buffs ?? [] });
  },
  fetchSaved: async () => {
    set({ saved: (await api.fetchSavedBuffs()) ?? [] });
  },
  fetchRound: async () => {
    const { round, initiative } = await roundApi.fetchRound();
    set({ round, initiative: initiative ?? null });
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
  saveBuffTemplate: async (saved) => {
    set({ saved: (await api.saveBuffTemplate(saved)) ?? [] });
  },
  deleteSavedBuff: async (id) => {
    set({ saved: (await api.deleteSavedBuff(id)) ?? [] });
  },
  advanceRound: async () => {
    const res = await roundApi.advanceRound();
    set({ round: res.roundNumber, initiative: res.initiative ?? null });
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
  passTime: async (hours) => {
    const res = await roundApi.passTime(hours);
    set({ round: res.roundNumber, buffs: res.activeBuffs ?? [] });
    return res;
  },
  rerollInitiative: async () => {
    set({ initiative: await roundApi.rerollInitiative() });
  },
}));
