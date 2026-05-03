import { create } from 'zustand';
import type { Character } from '@/types/character';
import { fetchCharacter } from '@/api/character';

interface CharacterStore {
  character: Character | null;
  loading: boolean;
  error: string | null;
  loadCharacter: () => Promise<void>;
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
}));
