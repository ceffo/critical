import { create } from "zustand";
import { useRollStore } from "./roll";

/**
 * UI state of the result card and the journal (no game state): the journal entry shown in the
 * card (null: the latest result), whether the attack's calculation details
 * are open.
 */
interface JournalStore {
  selectedId: number | null;
  detailOpen: boolean;
  select: (id: number) => void;
  /** Back to the latest result ("Latest"). */
  showLatest: () => void;
  setDetailOpen: (open: boolean) => void;
}

export const useJournalStore = create<JournalStore>((set) => ({
  selectedId: null,
  detailOpen: false,
  select: (id) => set({ selectedId: id }),
  showLatest: () => set({ selectedId: null }),
  setDetailOpen: (detailOpen) => set({ detailOpen }),
}));

// A new result (a roll, a cast, a correction) brings the card back to the latest result.
useRollStore.subscribe((s, prev) => {
  if (s.last !== prev.last) useJournalStore.getState().showLatest();
});
