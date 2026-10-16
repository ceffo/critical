import { create } from "zustand";
import type { LogFilter } from "@/lib/filCard";
import { useRollStore } from "./roll";

/**
 * UI state of the result panel and journal (no game state): the journal entry shown in the detail card (null:
 * the latest result), the journal filter and whether the attack's calculation details are open.
 */
interface FilStore {
  selectedId: number | null;
  filter: LogFilter;
  detailOpen: boolean;
  select: (id: number) => void;
  /** Back to the latest result ("Latest"). */
  showLatest: () => void;
  setFilter: (filter: LogFilter) => void;
  setDetailOpen: (open: boolean) => void;
}

export const useFilStore = create<FilStore>((set) => ({
  selectedId: null,
  filter: "all",
  detailOpen: false,
  select: (id) => set({ selectedId: id }),
  showLatest: () => set({ selectedId: null }),
  setFilter: (filter) => set({ filter }),
  setDetailOpen: (detailOpen) => set({ detailOpen }),
}));

// A new result (a roll, a cast, a correction) brings the card back to the latest result.
useRollStore.subscribe((s, prev) => {
  if (s.last !== prev.last) useFilStore.getState().showLatest();
});
