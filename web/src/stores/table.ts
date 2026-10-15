import { create } from "zustand";
import { appendHpEvent, undoLastEvent } from "@/api/hp";
import { ApiError } from "@/api/http";
import { amendLogEntry, voidLogEntry } from "@/api/log";
import { useHpStore } from "./hp";
import { useLogStore } from "./log";
import { useBuffsStore } from "./buffs";
import { useItemsStore } from "./items";

/** Something to tell the player after an action (rendered with i18n by the Notices component). */
export type Notice = { kind: "expired"; names: string[] };

interface TableStore {
  /** Last server (or network) error message; shown verbatim, never translated. */
  error: string | null;
  notice: Notice | null;
  busy: boolean;
  clearError: () => void;
  clearNotice: () => void;
  /** Reload everything an action can change: HP, action log, buffs, round, effective STR, item counts, weapons. */
  refresh: () => Promise<void>;
  /**
   * Run a mutation, then refresh the table state. Errors are caught into `error`.
   * Resolves true on success.
   */
  run: (action: () => Promise<unknown>) => Promise<boolean>;
  /**
   * Like `run` for a form: resolves null on success, or the ApiError when the server rejected a
   * field (shown by the form under that field, not in the global error). Other failures go to `error`.
   */
  submit: (action: () => Promise<unknown>) => Promise<ApiError | null>;
  recordHp: (delta: number, source: string) => Promise<boolean>;
  undoHp: () => Promise<boolean>;
  /** Set an entry's note (the only amendable field). */
  amendEntry: (id: number, note: string) => Promise<boolean>;
  voidEntry: (id: number) => Promise<boolean>;
  nextRound: () => Promise<boolean>;
  /** Pop the round's initiative entry and roll a new one (refused when another entry is newer). */
  rerollInitiative: () => Promise<boolean>;
  endCombat: () => Promise<boolean>;
  setRound: (round: number) => Promise<boolean>;
  /** Fast-forward hours of game time (timed buffs count down, round to 0). */
  passTime: (hours: number) => Promise<boolean>;
}

function expiredNotice(expired: { name: string }[] | undefined): Notice | null {
  return expired && expired.length > 0
    ? { kind: "expired", names: expired.map((b) => b.name) }
    : null;
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

let refreshQueue: Promise<void> = Promise.resolve();

export const useTableStore = create<TableStore>((set, get) => ({
  error: null,
  notice: null,
  busy: false,
  clearError: () => set({ error: null }),
  clearNotice: () => set({ notice: null }),

  // Refreshes run one after the other: when two actions overlap, an older refresh response
  // can no longer land after (and overwrite) the newer one.
  refresh: () => {
    const next = refreshQueue.then(async () => {
      try {
        await Promise.all([
          useHpStore.getState().fetchHp(),
          useLogStore.getState().fetchLog(),
          useBuffsStore.getState().fetchBuffs(),
          useBuffsStore.getState().fetchSaved(),
          useBuffsStore.getState().fetchRound(),
          useBuffsStore.getState().fetchStrength(),
          useItemsStore.getState().fetchItems(),
          useItemsStore.getState().fetchWeapons(),
        ]);
      } catch (e) {
        set({ error: message(e) });
      }
    });
    refreshQueue = next;
    return next;
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

  submit: async (action) => {
    set({ busy: true, error: null });
    let rejected: ApiError | null = null;
    try {
      await action();
    } catch (e) {
      if (e instanceof ApiError && e.field) rejected = e;
      else set({ error: message(e) });
    }
    await get().refresh();
    set({ busy: false });
    return rejected;
  },

  recordHp: (delta, source) =>
    get().run(async () =>
      useHpStore.getState().setHp(await appendHpEvent(delta, source)),
    ),
  undoHp: () =>
    get().run(async () => useHpStore.getState().setHp(await undoLastEvent())),
  amendEntry: (id, note) => get().run(() => amendLogEntry(id, note)),
  // A void can bring expired buffs back: an "expired" notice would now be stale.
  voidEntry: (id) =>
    get().run(async () => {
      await voidLogEntry(id);
      set({ notice: null });
    }),
  // A round change leaves buffs alone, but an "expired" notice is stale after either.
  endCombat: () =>
    get().run(async () => {
      await useBuffsStore.getState().resetRound();
      set({ notice: null });
    }),
  setRound: (round) =>
    get().run(async () => {
      await useBuffsStore.getState().setRound(round);
      set({ notice: null });
    }),
  nextRound: () =>
    get().run(async () => {
      const res = await useBuffsStore.getState().advanceRound();
      set({ notice: expiredNotice(res.expiredBuffs) });
    }),
  rerollInitiative: () => get().run(() => useBuffsStore.getState().rerollInitiative()),
  passTime: (hours) =>
    get().run(async () => {
      const res = await useBuffsStore.getState().passTime(hours);
      set({ notice: expiredNotice(res.expiredBuffs) });
    }),
}));
