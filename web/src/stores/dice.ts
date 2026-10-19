import { create } from "zustand";
import {
  addFavourite,
  deleteFavourite,
  fetchFavourites,
  reorderFavourites,
  rollDice,
} from "@/api/dice";
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
  /** Moves a favourite one place earlier (delta -1) or later (+1); the server keeps the order. */
  moveFavourite: (id: number, delta: -1 | 1) => Promise<void>;
}

/** The dice composer (free rolls and favourites), all rolled and stored by the server. */
export const useDiceStore = create<DiceStore>((set, get) => {
  const edit = (formula: string) => set({ formula, error: null });
  // Run by the action module: a refused formula is this store's own error, shown by the composer;
  // any other failure goes to the action module's error notice. The favourites are kept in the
  // background (quiet: no busy controls). Resolves true on success.
  const write = async (action: () => Promise<unknown>, quiet = true) => {
    let failed = false;
    const rejected = await useTableStore.getState().submit(
      async () => {
        try {
          await action();
        } catch (e) {
          failed = true;
          throw e;
        }
      },
      { quiet },
    );
    if (rejected?.field === "formula") set({ error: rejected.message });
    else if (rejected) useTableStore.getState().fail(rejected);
    return !failed;
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
      let entry: number | undefined;
      try {
        const ok = await write(async () => {
          const res = await rollDice(text);
          // The composer shows the server's normal form, ready to roll again.
          set({ formula: res.formula });
          entry = res.logEntryId;
        }, false);
        // The action module has read the journal again: the roll's entry is in it.
        if (ok && entry !== undefined) useFilStore.getState().select(entry);
        return ok;
      } finally {
        set({ rolling: false });
      }
    },
    fetchFavourites: async () => {
      await write(async () =>
        set({ favourites: (await fetchFavourites()).favourites }),
      );
    },
    saveFavourite: async () => {
      const text = get().formula;
      if (text.trim() === "") return false;
      return write(async () => {
        const fav = await addFavourite(text);
        set((s) => ({
          error: null,
          favourites: s.favourites.some((f) => f.id === fav.id)
            ? s.favourites
            : [...s.favourites, fav],
        }));
      });
    },
    removeFavourite: async (id) => {
      await write(async () =>
        set({ favourites: (await deleteFavourite(id)).favourites }),
      );
    },
    moveFavourite: async (id, delta) => {
      const ids = get().favourites.map((f) => f.id);
      const from = ids.indexOf(id);
      const to = from + delta;
      if (from < 0 || to < 0 || to >= ids.length) return;
      [ids[from], ids[to]] = [ids[to], ids[from]];
      await write(async () =>
        set({ favourites: (await reorderFavourites(ids)).favourites }),
      );
    },
  };
});
