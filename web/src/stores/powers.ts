import { create } from "zustand";
import type { Power, PowerFilterOptions } from "@/types/powers";
import {
  getPowerFilterOptions,
  NO_POWER_QUERY,
  searchPowers,
  type PowerQuery,
} from "@/api/powers";

/**
 * Power lookup state, beside the spell lookup's (stores/lookup.ts): the Spells tab shows the powers
 * while `active`. A module-level store: search, filters, rows and selection survive switching tabs;
 * a page reload clears them.
 */
interface PowerStore {
  /** Whether the lookup shows powers instead of spells. */
  active: boolean;
  query: PowerQuery;
  items: Power[];
  total: number;
  pages: number;
  /** The query the rows are for (queryKey); "" before the first search. */
  shownKey: string;
  loading: boolean;
  error: string | null;
  selectedId: string | null;
  options: PowerFilterOptions | null;

  setActive: (active: boolean) => void;
  setQuery: (patch: Partial<PowerQuery>) => void;
  reset: () => void;
  search: () => Promise<void>;
  loadMore: () => Promise<void>;
  select: (id: string) => void;
  move: (delta: number) => void;
  loadOptions: () => Promise<void>;
}

export function powerQueryKey(q: PowerQuery): string {
  return JSON.stringify({ ...q, q: q.q.trim() });
}

/** Whether anything differs from the default (the Reset link shows). */
export function isPowerQuerySet(q: PowerQuery): boolean {
  return powerQueryKey(q) !== powerQueryKey(NO_POWER_QUERY);
}

// Only the latest request may write: a slow older response must not overwrite it.
let latest = 0;

export const usePowerStore = create<PowerStore>((set, get) => ({
  active: false,
  query: NO_POWER_QUERY,
  items: [],
  total: 0,
  pages: 0,
  shownKey: "",
  loading: false,
  error: null,
  selectedId: null,
  options: null,

  setActive: (active) => set({ active }),

  setQuery: (patch) => set(({ query }) => ({ query: { ...query, ...patch } })),

  reset: () => set({ query: NO_POWER_QUERY, selectedId: null }),

  search: async () => {
    const { query } = get();
    const key = powerQueryKey(query);
    if (key === get().shownKey && !get().error) {
      latest++;
      set({ loading: false });
      return;
    }
    const seq = ++latest;
    set({ loading: true });
    try {
      const r = await searchPowers(query, 1);
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
      const r = await searchPowers(
        JSON.parse(shownKey) as PowerQuery,
        pages + 1,
      );
      if (seq !== latest || get().shownKey !== shownKey) return;
      const have = new Set(get().items.map((p) => p.id));
      set({
        items: [...get().items, ...r.items.filter((p) => !have.has(p.id))],
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
    const at = items.findIndex((p) => p.id === selectedId);
    const next = Math.max(0, Math.min(items.length - 1, at + delta));
    set({ selectedId: items[next].id });
    if (next >= items.length - 5 && items.length < total) void get().loadMore();
  },

  loadOptions: async () => {
    if (get().options) return;
    try {
      set({ options: await getPowerFilterOptions() });
    } catch {
      // The filters stay empty; the search itself still works.
    }
  },
}));
