import { create } from "zustand";
import type { Character, CharacterDetails } from "@/types/character";
import {
  fetchCharacter,
  patchCharacterDetails,
  rollLevelUp,
  takeNewClass,
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
   * before the details). Other failures go to the action module's error notice. Both are outside
   * the action log (ADR 0029 amendment step 4): the sheet is read again here, by hand. */
  saveDetails: (
    body: Partial<CharacterDetails>,
    name?: string,
  ) => Promise<ApiError | null>;
  /** Roll the hit points of `level`, the lowest pending one (ADR 0030 step 7), on the server, or
   * send `die`, the die rolled at the table. A transaction on the action stack, run by the action
   * module (refused or not, what it names is read again: a stale level shows the pending levels as
   * they are). Null on success, or the ApiError naming the rejected field; other failures go to the
   * action module's error notice. */
  rollLevelUp: (
    level: number,
    die?: number,
    cls?: string,
  ) => Promise<ApiError | null>;
  /** The character takes `cls` as a new class (ADR 0042 step 3), a transaction on the action stack
   * run by the action module, which reads again what it names (the sheet, the XP, the journal).
   * Null on success; a refusal is returned as its ApiError (the dialog words it), anything else goes
   * to the action module's error notice. */
  takeNewClass: (cls: string) => Promise<ApiError | null>;
}

export const useCharacterStore = create<CharacterStore>((set, get) => ({
  character: null,
  rollLevelUp: (level, die, cls) =>
    useTableStore.getState().submit(() => rollLevelUp(level, die, cls)),
  takeNewClass: async (cls) => {
    let refused: ApiError | null = null;
    await useTableStore.getState().submit(async () => {
      try {
        await takeNewClass(cls);
      } catch (e) {
        // A refusal (409 with a code) is the dialog's to word; nothing changed, nothing to read.
        if (e instanceof ApiError && e.status < 500) {
          refused = e;
          return;
        }
        throw e;
      }
    });
    return refused;
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
  saveDetails: (body, name) =>
    useTableStore.getState().submit(async () => {
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
      } catch (e) {
        // A name saved before the details were refused shows at once.
        if (renamed)
          await fetchCharacter().then(
            (character) => set({ character }),
            () => undefined,
          );
        throw e;
      }
    }),
}));
