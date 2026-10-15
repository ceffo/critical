import { create } from "zustand";
import type { LogEntry } from "@/types/log";
import { fetchLog } from "@/api/log";

interface LogStore {
  /** Newest first, as returned by GET /api/log. */
  entries: LogEntry[];
  /** The entry an undo pops (GET /api/log topId); null when nothing can be popped. */
  topId: number | null;
  loaded: boolean;
  fetchLog: () => Promise<void>;
}

export const useLogStore = create<LogStore>((set) => ({
  entries: [],
  topId: null,
  loaded: false,
  fetchLog: async () => {
    const { entries, topId } = await fetchLog();
    set({ entries, topId, loaded: true });
  },
}));
