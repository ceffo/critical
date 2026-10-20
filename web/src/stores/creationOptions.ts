import { create } from "zustand";
import { fetchCreationOptions, hasIdentityChoices } from "@/api/characters";
import type { CreationOptions } from "@/types/character";

interface CreationOptionsStore {
  /** null until read. */
  options: CreationOptions | null;
  /** Reads the options on first use and keeps them until the page reloads: they are reference
   * data, no write changes them. A failed read is not kept (the next call asks again). Throws on
   * failure: the caller shows its own retry. */
  load: () => Promise<CreationOptions>;
  /** `load`, refusing the options of a release that predates the alignment and sex lists (ADR
   * 0032, a rolling deploy, ADR 0024); those are dropped so that a retry asks again. */
  loadChoices: () => Promise<CreationOptions>;
}

let pending: Promise<CreationOptions> | null = null;

/** The creation options, one read per page load for the creation flow, the details card, the
 * roster and the pick editors. */
export const useCreationOptionsStore = create<CreationOptionsStore>(
  (set, get) => ({
    options: null,
    load: () => {
      const held = get().options;
      if (held) return Promise.resolve(held);
      pending ??= fetchCreationOptions()
        .then((o) => {
          set({ options: o });
          return o;
        })
        .finally(() => {
          pending = null;
        });
      return pending;
    },
    loadChoices: async () => {
      const o = await get().load();
      if (!hasIdentityChoices(o)) {
        set({ options: null });
        throw new Error("incomplete creation options");
      }
      return o;
    },
  }),
);
