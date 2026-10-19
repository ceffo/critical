import { create } from "zustand";
import { ApiError } from "@/api/http";
import { deleteXpEntry, fetchXp, patchXpEntry, postXpEntry } from "@/api/xp";
import type { XpEntryBody, XpLedger } from "@/types/xp";
import { useTableStore } from "./table";

interface XpStore {
  ledger: XpLedger | null;
  /** Entries added in this session, tinted in the ledger. */
  freshIds: ReadonlySet<number>;
  /** Throws on failure: read it through the action module's refresh. */
  fetchXp: () => Promise<void>;
  /** Each write resolves null on success, or the ApiError naming the rejected field (shown under
   * it). Other failures go to the action module's error notice. */
  add: (body: XpEntryBody) => Promise<ApiError | null>;
  update: (id: number, body: XpEntryBody) => Promise<ApiError | null>;
  remove: (id: number) => Promise<ApiError | null>;
}

/** The XP ledger (ADR 0015). Each write is a transaction on the action stack (step 4), run by the
 * action module: the journal shows the entry, its Undo targets it, and the sheet follows the XP. */
export const useXpStore = create<XpStore>((set, get) => {
  // The answer is the ledger: it is not read again.
  const write = (action: () => Promise<XpLedger>) =>
    useTableStore
      .getState()
      .submit(async () => set({ ledger: await action() }), { fresh: ["xp"] });
  return {
    ledger: null,
    freshIds: new Set(),
    fetchXp: async () => set({ ledger: await fetchXp() }),
    add: async (body) => {
      const before = new Set(get().ledger?.entries.map((e) => e.id));
      const rejected = await write(() => postXpEntry(body));
      if (!rejected) {
        const added = get().ledger?.entries.find((e) => !before.has(e.id));
        if (added) set({ freshIds: new Set(get().freshIds).add(added.id) });
      }
      return rejected;
    },
    update: (id, body) => write(() => patchXpEntry(id, body)),
    remove: (id) => write(() => deleteXpEntry(id)),
  };
});
