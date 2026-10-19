import { create } from "zustand";
import type { LogFilter } from "@/journal/entry";
import { useRollStore } from "./roll";

/**
 * UI state of the result card and the journal (no game state): the journal entry shown in the
 * card (null: the latest result), the journal filter and whether the attack's calculation details
 * are open.
 */
interface JournalStore {
  selectedId: number | null;
  filter: LogFilter;
  detailOpen: boolean;
  select: (id: number) => void;
  /** Back to the latest result ("Latest"). */
  showLatest: () => void;
  setFilter: (filter: LogFilter) => void;
  setDetailOpen: (open: boolean) => void;
}

export const useJournalStore = create<JournalStore>((set) => ({
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
  if (s.last !== prev.last) useJournalStore.getState().showLatest();
});
