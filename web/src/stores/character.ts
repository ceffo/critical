import { create } from "zustand";
import type { Character, CharacterDetails } from "@/types/character";
import { fetchCharacter, patchCharacterDetails } from "@/api/character";
import { ApiError } from "@/api/http";
import { useTableStore } from "./table";

interface CharacterStore {
  character: Character | null;
  loading: boolean;
  error: string | null;
  loadCharacter: () => Promise<void>;
  /** Saves the details: null on success, or the ApiError naming the rejected field (shown under
   * it). Other failures go to the table's error notice. */
  saveDetails: (body: Partial<CharacterDetails>) => Promise<ApiError | null>;
}

export const useCharacterStore = create<CharacterStore>((set) => ({
  character: null,
  loading: false,
  error: null,
  loadCharacter: async () => {
    set({ loading: true });
    try {
      const char = await fetchCharacter();
      set({ character: char, loading: false });
    } catch (e: unknown) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  saveDetails: async (body) => {
    try {
      const details = await patchCharacterDetails(body);
      set((s) => ({
        character: s.character ? { ...s.character, ...details } : s.character,
      }));
      return null;
    } catch (e) {
      if (e instanceof ApiError && e.field) return e;
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      });
      return null;
    }
  },
}));
