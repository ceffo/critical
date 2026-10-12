import { create } from "zustand";
import type { LogEntry } from "@/types/log";
import { fetchLog } from "@/api/log";

interface LogStore {
  /** Newest first, as returned by GET /api/log. */
  entries: LogEntry[];
  loaded: boolean;
  fetchLog: () => Promise<void>;
}

export const useLogStore = create<LogStore>((set) => ({
  entries: [],
  loaded: false,
  fetchLog: async () => {
    const entries = await fetchLog();
    set({ entries, loaded: true });
  },
}));
