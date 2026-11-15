/** The path prefix of another user's character, read-only (ADR 0029 step 11): `/view/<character id>`. */
export const VIEW_PREFIX = "/view/";

/** The header naming the character a read-only page shows (ADR 0029 step 11.3). */
export const VIEWED_HEADER = "X-Viewed-Character-Id";

/** The address of another user's character. */
export function viewPath(id: number): string {
  return `${VIEW_PREFIX}${id}`;
}

/**
 * The character a path views: its id, or null for any other path. A view is one page load (opened
 * and left through a full navigation), so the id of the page's path is the viewed character for
 * the page's whole life and none of the user's own state is mixed with it.
 */
export function viewedIdOf(path: string): number | null {
  const m = /^\/view\/([1-9]\d*)\/?$/.exec(path);
  return m ? Number(m[1]) : null;
}

/** The character the page views, null when it shows the user's own. */
export function viewedId(): number | null {
  return viewedIdOf(globalThis.location?.pathname ?? "");
}

/** Whether a request may leave a read-only page: a read, or the logout (the only write that touches no character). */
export function mayLeaveReadOnly(url: string, method: string | undefined) {
  const read = !method || method === "GET" || method === "HEAD";
  return read || url.startsWith("/api/auth/");
}
