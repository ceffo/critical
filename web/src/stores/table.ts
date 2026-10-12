import { create } from "zustand";
import { appendHpEvent, undoLastEvent } from "@/api/hp";
import { amendLogEntry, voidLogEntry } from "@/api/log";
import { useHpStore } from "./hp";
import { useLogStore } from "./log";
import { useBuffsStore } from "./buffs";

/** Something to tell the player after an action (rendered with i18n by the Notices component). */
export type Notice = { kind: "expired"; names: string[] };

interface TableStore {
  /** Last server (or network) error message; shown verbatim, never translated. */
  error: string | null;
  notice: Notice | null;
  busy: boolean;
  clearError: () => void;
  clearNotice: () => void;
  /** Reload everything an action can change: HP, action log, buffs, round. */
  refresh: () => Promise<void>;
  /**
   * Run a mutation, then refresh the table state. Errors are caught into `error`.
   * Resolves true on success.
   */
  run: (action: () => Promise<unknown>) => Promise<boolean>;
  recordHp: (delta: number, source: string) => Promise<boolean>;
  undoHp: () => Promise<boolean>;
  amendEntry: (id: number, delta: number, source: string) => Promise<boolean>;
  voidEntry: (id: number) => Promise<boolean>;
  nextRound: () => Promise<boolean>;
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export const useTableStore = create<TableStore>((set, get) => ({
  error: null,
  notice: null,
  busy: false,
  clearError: () => set({ error: null }),
  clearNotice: () => set({ notice: null }),

  refresh: async () => {
    try {
      await Promise.all([
        useHpStore.getState().fetchHp(),
        useLogStore.getState().fetchLog(),
        useBuffsStore.getState().fetchBuffs(),
        useBuffsStore.getState().fetchRound(),
      ]);
    } catch (e) {
      set({ error: message(e) });
    }
  },

  run: async (action) => {
    set({ busy: true, error: null });
    let ok = true;
    try {
      await action();
    } catch (e) {
      ok = false;
      set({ error: message(e) });
    }
    await get().refresh();
    set({ busy: false });
    return ok;
  },

  recordHp: (delta, source) =>
    get().run(async () => useHpStore.getState().setHp(await appendHpEvent(delta, source))),
  undoHp: () => get().run(async () => useHpStore.getState().setHp(await undoLastEvent())),
  amendEntry: (id, delta, source) => get().run(() => amendLogEntry(id, delta, source)),
  voidEntry: (id) => get().run(() => voidLogEntry(id)),
  nextRound: () =>
    get().run(async () => {
      const res = await useBuffsStore.getState().advanceRound();
      const expired = res.expiredBuffs ?? [];
      set({
        notice: expired.length > 0 ? { kind: "expired", names: expired.map((b) => b.name) } : null,
      });
    }),
}));
