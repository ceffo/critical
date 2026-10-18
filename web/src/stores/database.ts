import { create } from "zustand";

/**
 * Whether the database answers (ADR 0025 step 10.4), as the API's `database_unavailable` (503)
 * says. Kept by `api/http.ts`; the shell's banner shows it.
 */
interface DatabaseStore {
  /** The last request found the database unavailable, and nothing has answered since. */
  unavailable: boolean;
  /** A write got `database_unavailable`: it may or may not have been saved. Writes stay refused
   * until the sheet is reloaded from the database, so the journal shows whether it was. */
  uncertainWrite: boolean;
}

export const useDatabaseStore = create<DatabaseStore>(() => ({
  unavailable: false,
  uncertainWrite: false,
}));
