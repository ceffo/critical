import { create } from "zustand";
import { useCreationOptionsStore } from "./creationOptions";
import type { LanguageOption } from "@/types/character";

/**
 * The language catalogue (the creation options' `languages`), read from the creation options store (one read per page load) at start: the journal
 * names a language it knows only by code in the player's language.
 */
interface LanguagesStore {
  languages: LanguageOption[];
  load: () => Promise<void>;
}

export const useLanguagesStore = create<LanguagesStore>((set) => ({
  languages: [],
  load: async () => {
    try {
      set({
        languages:
          (await useCreationOptionsStore.getState().load()).languages ?? [],
      });
    } catch {
      // Codes show as they are (a release that predates the catalogue); the rest of the UI works.
    }
  },
}));
