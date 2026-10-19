import { create } from "zustand";
import type { Character, CharacterDetails } from "@/types/character";
import {
  fetchCharacter,
  patchCharacterDetails,
  rollLevelUp,
} from "@/api/character";
import { renameCharacter } from "@/api/characters";
import { ApiError } from "@/api/http";
import { useTableStore } from "./table";

interface CharacterStore {
  character: Character | null;
  loading: boolean;
  error: string | null;
  loadCharacter: () => Promise<void>;
  /** Saves the details, and the name first when `name` differs from the character's (Rename lives in
   * the details too, ADR 0034 step 7.2; the name is renamed on its own endpoint, ADR 0029 step 7.3):
   * null on success, or the ApiError naming the rejected field (shown under it; a refused name stops
   * before the details). Other failures go to the table's error notice. */
  saveDetails: (
    body: Partial<CharacterDetails>,
    name?: string,
  ) => Promise<ApiError | null>;
  /** A level-up roll in flight. */
  levelUpBusy: boolean;
  /** Roll the hit points of `level`, the lowest pending one (ADR 0030 step 7), on the server, or
   * send `die`, the die rolled at the table. A transaction on the action stack: the table is
   * refreshed with the character (its max HP and pending levels). Null on success, or the ApiError
   * naming the rejected field; other failures go to the table's error notice. */
  rollLevelUp: (level: number, die?: number) => Promise<ApiError | null>;
}

export const useCharacterStore = create<CharacterStore>((set, get) => ({
  character: null,
  levelUpBusy: false,
  rollLevelUp: async (level, die) => {
    set({ levelUpBusy: true });
    let rejected: ApiError | null = null;
    try {
      await rollLevelUp(level, die);
    } catch (e) {
      if (e instanceof ApiError && e.field) rejected = e;
      else
        useTableStore.setState({
          error: e instanceof Error ? e.message : String(e),
        });
    }
    // Refused or not, the sheet is read again: a stale level (another tab rolled it) shows the
    // pending levels as they are.
    await useTableStore.getState().refresh({ character: true });
    set({ levelUpBusy: false });
    return rejected;
  },
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
  saveDetails: async (body, name) => {
    let renamed = false;
    try {
      const c = get().character;
      if (c && name !== undefined && name !== c.name) {
        await renameCharacter(c.id, name);
        renamed = true;
      }
      await patchCharacterDetails(body);
      // The sheet is read again: the missing fields and the warnings are the server's to say.
      set({ character: await fetchCharacter() });
      return null;
    } catch (e) {
      // A name saved before the details were refused shows at once.
      if (renamed)
        await fetchCharacter().then(
          (character) => set({ character }),
          () => undefined,
        );
      if (e instanceof ApiError && e.field) return e;
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      });
      return null;
    }
  },
}));
