import { create } from "zustand";
import { ApiError } from "@/api/http";
import { addFavourite, deleteFavourite, fetchFavourites, rollDice } from "@/api/dice";
import { addDie, removeDie, stepConstant } from "@/lib/diceFormula";
import type { DiceFavourite } from "@/types/dice";
import { useFilStore } from "./fil";
import { useTableStore } from "./table";

interface DiceStore {
  /** The composer's text. Only built here: the server parses and rolls it. */
  formula: string;
  /** The server's reason when it refused the formula; cleared on the next edit. */
  error: string | null;
  favourites: DiceFavourite[];
  rolling: boolean;
  setFormula: (formula: string) => void;
  add: (sides: number) => void;
  remove: (sides: number) => void;
  step: (delta: number) => void;
  clear: () => void;
  /** Rolls the composer's text, or `formula` (a favourite, a reroll). Resolves true when rolled. */
  roll: (formula?: string) => Promise<boolean>;
  fetchFavourites: () => Promise<void>;
  /** Saves the composer's text on the server; resolves true when saved. */
  saveFavourite: () => Promise<boolean>;
  removeFavourite: (id: number) => Promise<void>;
}

/** The dice composer (free rolls and favourites), all rolled and stored by the server. */
export const useDiceStore = create<DiceStore>((set, get) => {
  const edit = (formula: string) => set({ formula, error: null });
  const fail = (e: unknown) => {
    if (e instanceof ApiError && e.field === "formula") set({ error: e.message });
    else useTableStore.setState({ error: e instanceof Error ? e.message : String(e) });
  };
  return {
    formula: "",
    error: null,
    favourites: [],
    rolling: false,
    setFormula: edit,
    add: (sides) => edit(addDie(get().formula, sides)),
    remove: (sides) => edit(removeDie(get().formula, sides)),
    step: (delta) => edit(stepConstant(get().formula, delta)),
    clear: () => edit(""),
    roll: async (formula) => {
      const text = formula ?? get().formula;
      if (get().rolling || text.trim() === "") return false;
      set({ rolling: true, error: null });
      useTableStore.getState().clearError();
      try {
        const res = await rollDice(text);
        // The composer shows the server's normal form, ready to roll again.
        set({ formula: res.formula });
        await useTableStore.getState().refresh();
        if (res.logEntryId !== undefined) useFilStore.getState().select(res.logEntryId);
        return true;
      } catch (e) {
        fail(e);
        return false;
      } finally {
        set({ rolling: false });
      }
    },
    fetchFavourites: async () => {
      try {
        set({ favourites: (await fetchFavourites()).favourites });
      } catch (e) {
        fail(e);
      }
    },
    saveFavourite: async () => {
      const text = get().formula;
      if (text.trim() === "") return false;
      try {
        const fav = await addFavourite(text);
        set((s) => ({
          error: null,
          favourites: s.favourites.some((f) => f.id === fav.id) ? s.favourites : [...s.favourites, fav],
        }));
        return true;
      } catch (e) {
        fail(e);
        return false;
      }
    },
    removeFavourite: async (id) => {
      try {
        set({ favourites: (await deleteFavourite(id)).favourites });
      } catch (e) {
        fail(e);
      }
    },
  };
});
