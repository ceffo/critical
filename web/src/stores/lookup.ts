import { create } from "zustand";
import type { Spell, SpellFilterOptions } from "@/types/spells";
import {
  getSpellFilterOptions,
  NO_QUERY,
  searchSpells,
  type SpellQuery,
} from "@/api/spells_db";

/** The lookup opens on Wizard spells. */
export const LOOKUP_DEFAULT: SpellQuery = { ...NO_QUERY, cls: "wizard" };

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

  setQuery: (patch: Partial<SpellQuery>) => void;
  reset: () => void;
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

  setQuery: (patch) =>
    set(({ query }) => {
      const next = { ...query, ...patch };
      // The dropdown lists spheres in Priest mode and schools otherwise: its value
      // means nothing in the other mode.
      if ((next.cls === "priest") !== (query.cls === "priest")) {
        next.school = "";
        next.sphere = "";
      }
      return { query: next };
    }),

  reset: () => set({ query: LOOKUP_DEFAULT, selectedId: null }),

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
    } catch (e: unknown) {
      if (seq !== latest) return;
      set({ error: (e as Error).message, loading: false });
    }
  },

  select: (id) => set({ selectedId: id }),

  move: (delta) => {
    const { items, selectedId, total } = get();
    if (items.length === 0) return;
    const at = items.findIndex((s) => s.id === selectedId);
    const next = Math.max(0, Math.min(items.length - 1, at + delta));
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
