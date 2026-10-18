import type { SessionState } from "@/api/auth";

/** The refusal codes the login page words, each its own message (`login.error.<code>`, en + fr). */
export const LOGIN_ERRORS = [
  "not_invited",
  "user_disabled",
  "session_expired",
  "email_unverified",
  "login_state_mismatch",
  "login_failed",
  "login_timeout",
  "database_unavailable",
] as const;

export type LoginError = (typeof LOGIN_ERRORS)[number];

/**
 * The i18n key of the login page's message for a refusal code (the `?error=` of the login callback's
 * redirect, ADR 0029 step 3.7, or the session's end): null without a code, `login.error.unknown` for
 * a code the web has no wording for.
 */
export function loginErrorKey(code: string | null | undefined): string | null {
  if (!code) return null;
  return (LOGIN_ERRORS as readonly string[]).includes(code)
    ? `login.error.${code}`
    : "login.error.unknown";
}

/** The login page's address for a session refusal code: it says the session ended when it did. */
export function loginUrl(code?: string): string {
  return code === "session_expired" ? "/login?error=session_expired" : "/login";
}

/** What the app shows at start-up. */
export type Start =
  | { show: "login"; error: string | null }
  | { show: "app" }
  | { show: "admin" }
  | { go: string };

/** The admin section's page (ADR 0029 step 1.3). */
export const ADMIN_PATH = "/admin";

/** True for the login page's path. */
export function isLoginPath(path: string): boolean {
  return path === "/login" || path.startsWith("/login/");
}

/**
 * What to show for a path and the session (ADR 0029 step 6): the login page only without a session
 * (a logged-in user who opens it goes to the app), the app only with one (without, the login page,
 * saying the session ended when it did), and the admin section only to an admin. With no answer from
 * the API the app or the page loads and shows the failure.
 */
export function startFor(
  path: string,
  search: string,
  session: SessionState,
): Start {
  if (isLoginPath(path)) {
    if (session.kind === "in") return { go: "/" };
    const error =
      new URLSearchParams(search).get("error") ??
      (session.kind === "out" && session.code === "session_expired"
        ? session.code
        : null);
    return { show: "login", error };
  }
  if (session.kind === "out") return { go: loginUrl(session.code) };
  if (path === ADMIN_PATH) {
    // The admin section is an admin's; a player who opens it goes to the app (the API refuses them anyway).
    if (session.kind === "in" && session.me.role !== "admin")
      return { go: "/" };
    return { show: "admin" };
  }
  return { show: "app" };
}
