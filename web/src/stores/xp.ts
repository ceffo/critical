import { create } from "zustand";
import { ApiError } from "@/api/http";
import { deleteXpEntry, fetchXp, patchXpEntry, postXpEntry } from "@/api/xp";
import type { XpEntryBody, XpLedger } from "@/types/xp";
import { useTableStore } from "./table";

interface XpStore {
  ledger: XpLedger | null;
  busy: boolean;
  /** Entries added in this session, tinted in the ledger. */
  freshIds: ReadonlySet<number>;
  fetchXp: () => Promise<void>;
  /** Each write resolves null on success, or the ApiError naming the rejected field (shown under
   * it). Other failures go to the table's error notice. */
  add: (body: XpEntryBody) => Promise<ApiError | null>;
  update: (id: number, body: XpEntryBody) => Promise<ApiError | null>;
  remove: (id: number) => Promise<ApiError | null>;
}

/** The XP ledger (ADR 0015): outside the action stack, so it never refreshes the table state. */
export const useXpStore = create<XpStore>((set, get) => {
  const write = async (action: () => Promise<XpLedger>): Promise<ApiError | null> => {
    set({ busy: true });
    try {
      set({ ledger: await action() });
      return null;
    } catch (e) {
      if (e instanceof ApiError && e.field) return e;
      useTableStore.setState({ error: e instanceof Error ? e.message : String(e) });
      return null;
    } finally {
      set({ busy: false });
    }
  };
  return {
    ledger: null,
    busy: false,
    freshIds: new Set(),
    fetchXp: async () => {
      try {
        set({ ledger: await fetchXp() });
      } catch (e) {
        useTableStore.setState({ error: e instanceof Error ? e.message : String(e) });
      }
    },
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
