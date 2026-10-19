import { create } from "zustand";
import { appendHpEvent } from "@/api/hp";
import { ApiError } from "@/api/http";
import { redoLogEntry, undoLogEntry } from "@/api/log";
import { useHpStore } from "./hp";
import { useLogStore } from "./log";
import { useBuffsStore } from "./buffs";
import { useCharacterStore } from "./character";
import { useItemsStore } from "./items";
import { useAcStore } from "./ac";
import { useRollStore } from "./roll";
import { useXpStore } from "./xp";

/** Something to tell the player after an action (rendered with i18n by the Notices component). */
/** Buffs that just expired, named at render time in the player's language (buffName). */
export type Notice = {
  kind: "expired";
  buffs: { name: string; spellId?: string }[];
};

interface RefreshOptions {
  /** The caller just set the XP ledger from a write's answer: not read again. */
  skipXp?: boolean;
  /** The action can have changed the XP total (an XP write, an undo or a redo): the character is
   * read again, since its level, THAC0, saves and slots follow it (ADR 0030 step 3). Not on every
   * action: GET /api/character is also the database probe, and a read retries while the
   * database is unavailable, so the refresh (and the busy controls) would wait on it. */
  character?: boolean;
}

interface TableStore {
  /** Last server (or network) error message; shown verbatim, never translated. */
  error: string | null;
  notice: Notice | null;
  busy: boolean;
  clearError: () => void;
  clearNotice: () => void;
  /** Reload everything an action can change: HP, action log, buffs, round, effective STR, item
   * counts, weapons, the AC, the XP ledger once loaded (`skipXp`: the caller just set it from a write's
   * answer), and the character when the action can have changed its XP (`character`). */
  refresh: (opts?: RefreshOptions) => Promise<void>;
  /**
   * Run a mutation, then refresh the table state. Errors are caught into `error`.
   * Resolves true on success.
   */
  run: (
    action: () => Promise<unknown>,
    opts?: RefreshOptions,
  ) => Promise<boolean>;
  /**
   * Like `run` for a form: resolves null on success, or the ApiError when the server rejected a
   * field (shown by the form under that field, not in the global error). Other failures go to `error`.
   */
  submit: (action: () => Promise<unknown>) => Promise<ApiError | null>;
  recordHp: (delta: number, source: string) => Promise<boolean>;
  /** Pop the top of the action stack, any kind, as last read (ADR 0010). False when the server
   * refused (another action landed first) or there was nothing to pop. */
  undo: () => Promise<boolean>;
  /** Replay the newest popped transaction, as last read (ADR 0019). False when the server refused
   * (another action landed or another client redid first) or there was nothing to redo. */
  redo: () => Promise<boolean>;
  nextRound: () => Promise<boolean>;
  /** Pop the round's initiative entry and roll a new one (refused when another entry is newer). */
  rerollInitiative: () => Promise<boolean>;
  endCombat: () => Promise<boolean>;
  setRound: (round: number) => Promise<boolean>;
  /** Fast-forward hours of game time (timed buffs count down, round to 0). */
  passTime: (hours: number) => Promise<boolean>;
}

function expiredNotice(
  expired: { name: string; spellId?: string }[] | undefined,
): Notice | null {
  return expired && expired.length > 0
    ? {
        kind: "expired",
        buffs: expired.map((b) => ({ name: b.name, spellId: b.spellId })),
      }
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
  refresh: (opts) => {
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
          // Derived from the pieces, the buffs and the sheet (ADR 0033 step 10): any action can move it.
          useAcStore.getState().fetchAc(),
          // The level, THAC0, saves and slots follow the XP (ADR 0030 step 3): an award or the
          // undo of one changes them, so the character is re-read with the rest.
          opts?.character ? useCharacterStore.getState().loadCharacter() : null,
          // An undo can pop an XP award (ADR 0015 step 4): re-read the ledger once it is shown.
          useXpStore.getState().ledger && !opts?.skipXp
            ? useXpStore.getState().fetchXp()
            : null,
        ]);
      } catch (e) {
        set({ error: message(e) });
      }
    });
    refreshQueue = next;
    return next;
  },

  run: async (action, opts) => {
    set({ busy: true, error: null });
    let ok = true;
    try {
      await action();
    } catch (e) {
      ok = false;
      set({ error: message(e) });
    }
    await get().refresh(opts);
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
  // One undo system (ADR 0018 step 3), the journal's Undo button and Ctrl+Z: the global undo
  // with the top this client last read, so a stale top (another client acted since) is refused
  // and nothing changes. A pop can bring expired buffs back (an "expired" notice would now be
  // stale) and can take back a correction (ADR 0010 step 6): the card then shows the latest roll
  // from the refreshed log, not this session's (corrected) roll response.
  undo: () => {
    const expected = useLogStore.getState().topId;
    if (expected === null) return Promise.resolve(false);
    return get().run(
      async () => {
        await undoLogEntry(expected);
        set({ notice: null });
        useRollStore.setState({ last: null });
      },
      { character: true },
    );
  },
  // Redo (ADR 0019 step 3), the journal's Redo button and Ctrl+Shift+Z: replays the transaction
  // this client last read as the redo target, so a stale one is refused and nothing changes. Like
  // an undo, it can change anything the card or a notice shows: both are re-read from the log.
  redo: () => {
    const expected = useLogStore.getState().redo?.id ?? null;
    if (expected === null) return Promise.resolve(false);
    return get().run(
      async () => {
        await redoLogEntry(expected);
        set({ notice: null });
        useRollStore.setState({ last: null });
      },
      { character: true },
    );
  },
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
  rerollInitiative: () =>
    get().run(() => useBuffsStore.getState().rerollInitiative()),
  passTime: (hours) =>
    get().run(async () => {
      const res = await useBuffsStore.getState().passTime(hours);
      set({ notice: expiredNotice(res.expiredBuffs) });
    }),
}));
