import { create } from "zustand";
import { appendHpEvent } from "@/api/hp";
import { ApiError, UNAVAILABLE } from "@/api/http";
import { redoLogEntry, undoLogEntry } from "@/api/log";
import { SLICES, slicesFor, takeChanges, type Slice } from "@/lib/stateKeys";
import { useHpStore } from "./hp";
import { useLogStore } from "./log";
import { useBuffsStore } from "./buffs";
import { useCharacterStore } from "./character";
import { useItemsStore } from "./items";
import { useAcStore } from "./ac";
import { useRollStore } from "./roll";
import { useSkillsStore } from "./skills";
import { useSpellStore } from "./spells";
import { useXpStore } from "./xp";
import type { EndCause, Ended } from "@/types/buffs";

/** Something to tell the player after an action, rendered with i18n by the Notices component: the
 * buffs that just expired, named at render time in the player's language (buffName), and what the
 * end effects of the rows that ended did (ADR 0004 step 21). */
export type Notice = {
  kind: "expired";
  buffs: { name: string; spellId?: string }[];
  ended?: Ended;
};

interface ActionOptions {
  /** Slices the action already set from its answer: not read again. */
  fresh?: readonly Slice[];
  /** A background action (the attack form's saves and reads): the busy flag is left alone and a
   * shown error is not cleared. */
  quiet?: boolean;
}

/**
 * The action module (ADR 0018 step 11): the one place that runs a write, owns the error, the notice
 * and the busy flag, and reads again what the write changed. The data stores hold the data and their
 * fetches only: none reads another store again or sets another store's error. What a write changed
 * is what the server named in its answer (lib/stateKeys); an unknown change reads every slice again.
 */
interface TableStore {
  /** Last server (or network) error message; shown verbatim, never translated. */
  error: string | null;
  notice: Notice | null;
  busy: boolean;
  clearError: () => void;
  clearNotice: () => void;
  /** Shows a failure in the error notice. */
  fail: (e: unknown) => void;
  /** Read `slices` again, or, without them, every slice as it is loaded (a slice no panel has
   * loaded yet stays unread). Errors go to `error`. */
  refresh: (slices?: readonly Slice[]) => Promise<void>;
  /**
   * Run a mutation, then read again what the server said it changed. Errors are caught into
   * `error`. Resolves true on success.
   */
  run: (
    action: () => Promise<unknown>,
    opts?: ActionOptions,
  ) => Promise<boolean>;
  /**
   * Like `run` for a form: resolves null on success, or the ApiError when the server rejected a
   * field (shown by the form under that field, not in the global error). Other failures go to `error`.
   */
  submit: (
    action: () => Promise<unknown>,
    opts?: ActionOptions,
  ) => Promise<ApiError | null>;
  recordHp: (delta: number, source: string) => Promise<boolean>;
  /** Pop the top of the action stack, any kind, as last read (ADR 0010). False when the server
   * refused (another action landed first) or there was nothing to pop. */
  undo: () => Promise<boolean>;
  /** Replay the newest popped transaction, as last read (ADR 0019). False when the server refused
   * (another action landed or another client redid first) or there was nothing to redo. */
  redo: () => Promise<boolean>;
  nextRound: () => Promise<boolean>;
  /** Stops a row; `cause` is how it ended when its spell has end effects (ADR 0004 step 21). What
   * they did becomes the notice. */
  dismissBuff: (id: string, cause?: EndCause) => Promise<boolean>;
  /** Pop the round's initiative entry and roll a new one (refused when another entry is newer). */
  rerollInitiative: () => Promise<boolean>;
  endCombat: () => Promise<boolean>;
  setRound: (round: number) => Promise<boolean>;
  /** Fast-forward hours of game time (timed buffs count down, round to 0). */
  passTime: (hours: number) => Promise<boolean>;
}

/**
 * The fetch of each slice, exhaustive by type. `loaded` says whether a panel has read the slice
 * yet: a refresh of every slice skips the ones no panel shows.
 */
const fetchers: Record<
  Slice,
  { fetch: () => Promise<void>; loaded?: () => boolean }
> = {
  log: { fetch: () => useLogStore.getState().fetchLog() },
  hp: { fetch: () => useHpStore.getState().fetchHp() },
  buffs: { fetch: () => useBuffsStore.getState().fetchBuffs() },
  savedBuffs: { fetch: () => useBuffsStore.getState().fetchSaved() },
  round: { fetch: () => useBuffsStore.getState().fetchRound() },
  strength: { fetch: () => useBuffsStore.getState().fetchStrength() },
  items: { fetch: () => useItemsStore.getState().fetchItems() },
  weapons: { fetch: () => useItemsStore.getState().fetchWeapons() },
  ac: { fetch: () => useAcStore.getState().fetchAc() },
  slots: { fetch: () => useSpellStore.getState().fetchSlots() },
  spellInfo: {
    fetch: () => useSpellStore.getState().fetchInfo(),
    loaded: () => useSpellStore.getState().infoLoaded,
  },
  skills: {
    fetch: () => useSkillsStore.getState().fetchSkills(),
    loaded: () => useSkillsStore.getState().skills !== null,
  },
  // GET /api/character is also the database probe, and a read retries while the database is
  // unavailable: it is read only when a change can have moved it (the level, the budget).
  character: { fetch: () => useCharacterStore.getState().loadCharacter() },
  xp: {
    fetch: () => useXpStore.getState().fetchXp(),
    loaded: () => useXpStore.getState().ledger !== null,
  },
};

function expiredNotice(
  expired: { name: string; spellId?: string }[] | undefined,
  ended?: Ended,
): Notice | null {
  return (expired && expired.length > 0) || ended
    ? {
        kind: "expired",
        buffs: (expired ?? []).map((b) => ({
          name: b.name,
          spellId: b.spellId,
        })),
        ended,
      }
    : null;
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

let refreshQueue: Promise<void> = Promise.resolve();

export const useTableStore = create<TableStore>((set, get) => {
  // Refreshes run one after the other: when two actions overlap, an older refresh response
  // can no longer land after (and overwrite) the newer one.
  const read = (slices: Iterable<Slice>): Promise<void> => {
    const list = [...slices];
    const next = refreshQueue.then(async () => {
      if (list.length === 0) return;
      try {
        await Promise.all(list.map((s) => fetchers[s].fetch()));
      } catch (e) {
        set({ error: message(e) });
      }
    });
    refreshQueue = next;
    return next;
  };
  const shown = (s: Slice) => fetchers[s].loaded?.() ?? true;
  // What the writes answered since the last action changed, but what the action set from its
  // answer and the slices no panel has loaded. A refused input (400) names nothing and reads
  // nothing; a stale refusal (404, 409), a server error or a lost answer is unknown and reads
  // everything, so the player sees the state that refused it (a stale undo's new top) (noteAnswer).
  // One the database could not serve reads nothing: the reads would wait for it (and the busy
  // controls with them), and the sheet is read again once it answers (api/http.ts setReload).
  const changed = (failure: unknown, opts?: ActionOptions) => {
    const answered = takeChanges();
    if (failure instanceof ApiError && failure.code === UNAVAILABLE) return [];
    const slices = slicesFor(answered);
    if (failure === undefined)
      for (const s of opts?.fresh ?? []) slices.delete(s);
    return [...slices].filter(shown);
  };
  const start = (opts?: ActionOptions) => {
    if (!opts?.quiet) set({ busy: true, error: null });
  };
  const end = (opts?: ActionOptions) => {
    if (!opts?.quiet) set({ busy: false });
  };

  return {
    error: null,
    notice: null,
    busy: false,
    clearError: () => set({ error: null }),
    clearNotice: () => set({ notice: null }),
    fail: (e) => set({ error: message(e) }),

    refresh: (slices) => read(slices ?? SLICES.filter(shown)),

    run: async (action, opts) => {
      start(opts);
      let failure: unknown = undefined;
      try {
        await action();
      } catch (e) {
        failure = e ?? new Error("failed");
        set({ error: message(e) });
      }
      await read(changed(failure, opts));
      end(opts);
      return failure === undefined;
    },

    submit: async (action, opts) => {
      start(opts);
      let rejected: ApiError | null = null;
      let failure: unknown = undefined;
      try {
        await action();
      } catch (e) {
        failure = e ?? new Error("failed");
        if (e instanceof ApiError && e.field) rejected = e;
        else set({ error: message(e) });
      }
      await read(changed(failure, opts));
      end(opts);
      return rejected;
    },

    recordHp: (delta, source) =>
      get().run(
        async () =>
          useHpStore.getState().setHp(await appendHpEvent(delta, source)),
        { fresh: ["hp"] },
      ),
    // One undo system (ADR 0018 step 3), the journal's Undo button and Ctrl+Z: the global undo
    // with the top this client last read, so a stale top (another client acted since) is refused
    // and nothing changes. A pop can bring expired buffs back (an "expired" notice would now be
    // stale) and can take back a correction (ADR 0010 step 6): the card then shows the latest roll
    // from the refreshed log, not this session's (corrected) roll response. What the pop restored
    // is read again by the keys it names, like any action (ADR 0019: undo and redo are two more).
    undo: () => {
      const expected = useLogStore.getState().topId;
      if (expected === null) return Promise.resolve(false);
      return get().run(async () => {
        await undoLogEntry(expected);
        set({ notice: null });
        useRollStore.setState({ last: null });
      });
    },
    // Redo (ADR 0019 step 3), the journal's Redo button and Ctrl+Shift+Z: replays the transaction
    // this client last read as the redo target, so a stale one is refused and nothing changes. Like
    // an undo, it can change anything the card or a notice shows: both are re-read from the log.
    redo: () => {
      const expected = useLogStore.getState().redo?.id ?? null;
      if (expected === null) return Promise.resolve(false);
      return get().run(async () => {
        await redoLogEntry(expected);
        set({ notice: null });
        useRollStore.setState({ last: null });
      });
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
        set({ notice: expiredNotice(res.expiredBuffs, res.ended) });
      }),
    dismissBuff: (id, cause) =>
      get().run(async () => {
        const ended = await useBuffsStore.getState().dismissBuff(id, cause);
        set({ notice: expiredNotice([], ended) });
      }),
    rerollInitiative: () =>
      get().run(() => useBuffsStore.getState().rerollInitiative()),
    passTime: (hours) =>
      get().run(async () => {
        const res = await useBuffsStore.getState().passTime(hours);
        set({ notice: expiredNotice(res.expiredBuffs, res.ended) });
      }),
  };
});
