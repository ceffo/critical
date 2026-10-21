import type { ChangelogPage, ChangelogVersion } from "@/types/changelog";

/** What the panel shows: the versions loaded so far and where the loading stands. */
export interface PagesState {
  versions: ChangelogVersion[];
  /** A request is in flight. */
  loading: boolean;
  /** The last request failed (its page is not loaded; `loadMore` asks for it again). */
  failed: boolean;
  /** The server named no further page. */
  done: boolean;
}

/**
 * The paginated changelog read by infinite scroll: `loadMore` asks for the next page unless one is
 * already in flight or the list is complete (so a scroll event firing many times asks once), `loadMore`
 * after a failure is the retry. Plain subscribe / getSnapshot, for `useSyncExternalStore`.
 */
export function createChangelogPages(
  fetchPage: (cursor?: string) => Promise<ChangelogPage>,
) {
  let state: PagesState = {
    versions: [],
    loading: false,
    failed: false,
    done: false,
  };
  let cursor: string | undefined;
  const listeners = new Set<() => void>();
  const set = (next: Partial<PagesState>) => {
    state = { ...state, ...next };
    listeners.forEach((l) => l());
  };

  async function loadMore() {
    if (state.loading || state.done) return;
    set({ loading: true, failed: false });
    try {
      const page = await fetchPage(cursor);
      cursor = page.nextCursor || undefined;
      set({
        versions: [...state.versions, ...page.versions],
        loading: false,
        done: !page.nextCursor,
      });
    } catch {
      set({ loading: false, failed: true });
    }
  }

  return {
    loadMore,
    getSnapshot: () => state,
    subscribe(l: () => void) {
      listeners.add(l);
      return () => void listeners.delete(l);
    },
  };
}
