import { create } from "zustand";
import type {
  BuffEdit,
  BuffEffect,
  BuffList,
  Initiative,
  ItemPower,
  RoundAdvanceResponse,
  SavedBuff,
  Strength,
} from "../types/buffs";
import * as api from "../api/buffs";
import * as roundApi from "../api/round";

interface BuffsStore {
  buffs: BuffEffect[];
  /** The carried items' powers that are on, beside the buffs (ADR 0016 step 13). */
  powers: ItemPower[];
  round: number;
  /** The round's initiative, from the server; null out of combat or until rolled. */
  initiative: Initiative | null;
  /** Saved effect templates, oldest first. */
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

/** The store's slice of an effects list answer (null: an empty body). */
function fromList(list: BuffList | null | undefined) {
  return { buffs: list?.buffs ?? [], powers: list?.powers ?? [] };
}

export const useBuffsStore = create<BuffsStore>((set) => ({
  buffs: [],
  powers: [],
  round: 0,
  initiative: null,
  saved: [],
  strength: null,
  fetchBuffs: async () => {
    set(fromList(await api.fetchActiveBuffs()));
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
    set(fromList(await api.activateBuff(buff)));
  },
  editBuff: async (id, edit) => {
    set(fromList(await api.editBuff(id, edit)));
  },
  dismissBuff: async (id) => {
    set(fromList(await api.dismissBuff(id)));
  },
  triggerBuff: async (id) => {
    set(fromList(await api.triggerBuff(id)));
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
