import { request } from "@/api/http";
import type { ChangelogPage } from "@/types/changelog";

/** Versions asked per page: a few screens of the panel, so the next page rarely shows as a wait. */
export const PAGE_SIZE = 10;

/** One page of the changelog (session-protected, ADR 0029 step 6.1); `cursor` is the previous page's `nextCursor`. */
export function fetchChangelog(cursor?: string): Promise<ChangelogPage> {
  const q = new URLSearchParams({ limit: String(PAGE_SIZE) });
  if (cursor) q.set("cursor", cursor);
  return request<ChangelogPage>(`/api/changelog?${q}`);
}
