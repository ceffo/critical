import { create } from "zustand";
import type { FreeSpell } from "@/types/character";
import type { Spell, SpellFilterOptions } from "@/types/spells";
import {
  getSpellFilterOptions,
  NO_QUERY,
  searchSpells,
  type SpellQuery,
} from "@/api/spells_db";

/** The lookup opens on Wizard spells. */
export const LOOKUP_DEFAULT: SpellQuery = { ...NO_QUERY, cls: "wizard" };

/** The query a free spell's link opens (ADR 0030 step 20.8): the wizard spells of its spell level. */
export function freeSpellQuery(fs: FreeSpell): SpellQuery {
  return { ...LOOKUP_DEFAULT, levels: [fs.spellLevel] };
}

/**
 * Spell Lookup state. A module-level store: search, filters, loaded rows and the
 * selection survive switching tabs (the panel unmounts); a page reload clears them.
 */
interface LookupStore {
  query: SpellQuery;
  /** Rows loaded so far (pages 1..pages of the query in `shownKey`). */
  items: Spell[];
  /** Every match of the query, across all pages. */
  total: number;
  pages: number;
  /** The query the rows are for (queryKey); "" before the first search. */
  shownKey: string;
  loading: boolean;
  error: string | null;
  selectedId: string | null;
  options: SpellFilterOptions | null;
  /** The free spell the lookup was opened for (ADR 0030 step 20.8): Ajouter claims it while set.
   * Changing the class or the levels, or Reset, drops it. */
  grant: FreeSpell | null;

  setQuery: (patch: Partial<SpellQuery>) => void;
  reset: () => void;
  /** Opens the lookup on a free spell's spell level, its Ajouter claiming that free spell. */
  openFreeSpell: (fs: FreeSpell) => void;
  /** Drops the free spell being claimed (claimed, or no longer owed). */
  clearGrant: () => void;
  /** First page of the current query, unless its rows are already shown. Selects the first row. */
  search: () => Promise<void>;
  /** Next page of the current rows' query, if any is left. */
  loadMore: () => Promise<void>;
  select: (id: string) => void;
  /** Move the selection by delta rows (keyboard up/down). */
  move: (delta: number) => void;
  loadOptions: () => Promise<void>;
}

export function queryKey(q: SpellQuery): string {
  return JSON.stringify({ ...q, q: q.q.trim() });
}

/** Whether anything differs from the default (the Reset link shows). */
export function isQuerySet(q: SpellQuery): boolean {
  return queryKey(q) !== queryKey(LOOKUP_DEFAULT);
}

// Only the latest request may write: a slow older response must not overwrite it.
let latest = 0;

// Rows of keyboard movement asked for past the last loaded row while the next page loads; applied
// when it lands, dropped when the rows change under them.
let pendingMove = 0;

export const useLookupStore = create<LookupStore>((set, get) => ({
  query: LOOKUP_DEFAULT,
  items: [],
  total: 0,
  pages: 0,
  shownKey: "",
  loading: false,
  error: null,
  selectedId: null,
  options: null,
  grant: null,

  setQuery: (patch) =>
    set(({ query, grant }) => {
      const next = { ...query, ...patch };
      // The dropdown lists spheres in Priest mode and schools otherwise: its value
      // means nothing in the other mode.
      if ((next.cls === "priest") !== (query.cls === "priest")) {
        next.school = "";
        next.sphere = "";
      }
      // The free spell is claimed among its own spell level's wizard spells only.
      const keep =
        grant !== null &&
        next.cls === query.cls &&
        next.levels.join() === query.levels.join();
      return { query: next, grant: keep ? grant : null };
    }),

  reset: () => {
    pendingMove = 0;
    set({ query: LOOKUP_DEFAULT, selectedId: null, grant: null });
  },

  openFreeSpell: (fs) => {
    pendingMove = 0;
    set({ query: freeSpellQuery(fs), grant: fs });
  },

  clearGrant: () => set({ grant: null }),

  search: async () => {
    const { query } = get();
    const key = queryKey(query);
    if (key === get().shownKey && !get().error) {
      // Back to the rows on screen: drop any search still in flight.
      latest++;
      set({ loading: false });
      return;
    }
    const seq = ++latest;
    pendingMove = 0;
    set({ loading: true });
    try {
      const r = await searchSpells(query, 1);
      if (seq !== latest) return;
      set({
        items: r.items,
        total: r.total,
        pages: 1,
        shownKey: key,
        loading: false,
        error: null,
        selectedId: r.items[0]?.id ?? null,
      });
    } catch (e: unknown) {
      if (seq !== latest) return;
      set({ error: (e as Error).message, loading: false });
    }
  },

  loadMore: async () => {
    const { items, total, pages, shownKey, loading } = get();
    if (loading || !shownKey || items.length >= total) return;
    const seq = ++latest;
    set({ loading: true });
    try {
      const r = await searchSpells(
        JSON.parse(shownKey) as SpellQuery,
        pages + 1,
      );
      if (seq !== latest || get().shownKey !== shownKey) return;
      const have = new Set(get().items.map((s) => s.id));
      set({
        items: [...get().items, ...r.items.filter((s) => !have.has(s.id))],
        total: r.total,
        pages: pages + 1,
        loading: false,
        error: null,
      });
      const carried = pendingMove;
      pendingMove = 0;
      if (carried !== 0) get().move(carried);
    } catch (e: unknown) {
      if (seq !== latest) return;
      pendingMove = 0;
      set({ error: (e as Error).message, loading: false });
    }
  },

  select: (id) => {
    pendingMove = 0;
    set({ selectedId: id });
  },

  move: (delta) => {
    const { items, selectedId, total } = get();
    if (items.length === 0) return;
    const at = items.findIndex((s) => s.id === selectedId);
    // Moves already asked for past the loaded rows count from where the selection will be.
    const want = at + pendingMove + delta;
    const next = Math.max(0, Math.min(items.length - 1, want));
    // Past the loaded rows with more to come: the rest of the move waits for the next page.
    pendingMove = want > next && items.length < total ? want - next : 0;
    set({ selectedId: items[next].id });
    if (next >= items.length - 5 && items.length < total) void get().loadMore();
  },

  loadOptions: async () => {
    if (get().options) return;
    try {
      set({ options: await getSpellFilterOptions() });
    } catch {
      // The dropdown stays empty; the search itself still works.
    }
  },
}));
