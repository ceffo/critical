import { create } from "zustand";
import type { Item } from "@/types/items";
import type { Weapon } from "@/types/combat";
import { fetchItems } from "@/api/items";
import { fetchWeapons } from "@/api/combat";

interface ItemsStore {
  items: Item[];
  weapons: Weapon[];
  /** The weapons were read at least once: an empty list is then an answer, not a read in flight. */
  weaponsLoaded: boolean;
  fetchItems: () => Promise<void>;
  fetchWeapons: () => Promise<void>;
  /** The items as a write outside the action log answered them (it names no state key). */
  setItems: (items: Item[]) => void;
}

/** Counted items (ammo counts change with attacks and voids) and the weapons catalog. */
export const useItemsStore = create<ItemsStore>((set) => ({
  items: [],
  weapons: [],
  weaponsLoaded: false,
  fetchItems: async () => set({ items: (await fetchItems()) ?? [] }),
  fetchWeapons: async () =>
    set({ weapons: (await fetchWeapons()) ?? [], weaponsLoaded: true }),
  setItems: (items) => set({ items }),
}));
