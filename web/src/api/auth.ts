import { post } from "./http";

/** The logged-in user (`GET /api/auth/me`, ADR 0029 step 4.6). */
export interface Me {
  id: string;
  email: string;
  role: "admin" | "player";
  root: boolean;
}

/** How the server lets a user log in (`GET /api/auth/options`): Google with OAuth settings, else the dev login. */
export interface LoginOptions {
  google: boolean;
  devLogin: boolean;
}

/** Where the login page's Google button goes: the server starts the Google login (ADR 0029 step 3.1). */
export const GOOGLE_START = "/api/auth/google/start";

/** The login page (ADR 0029 step 6.1), the one page shown without a session. */
export const LOGIN_PATH = "/login";

/** The session as the app starts: a user, none (with the server's code), or no answer from the API. */
export type SessionState =
  { kind: "in"; me: Me } | { kind: "out"; code?: string } | { kind: "down" };

/** The current session, read once at start-up. A 401 is no session: `session_expired` when the browser had one. */
export async function readSession(): Promise<SessionState> {
  try {
    const res = await fetch("/api/auth/me");
    if (res.ok) return { kind: "in", me: (await res.json()) as Me };
    if (res.status !== 401) return { kind: "down" };
    const body = (await res.json().catch(() => ({}))) as { code?: unknown };
    return {
      kind: "out",
      code: typeof body.code === "string" ? body.code : undefined,
    };
  } catch {
    return { kind: "down" };
  }
}

/** The login methods the server offers; Google alone when the API does not answer. */
export async function loginOptions(): Promise<LoginOptions> {
  try {
    const res = await fetch("/api/auth/options");
    if (res.ok) return (await res.json()) as LoginOptions;
  } catch {
    // the page still offers Google
  }
  return { google: true, devLogin: false };
}

/** The dev login (ADR 0029 step 8.1): a session for the root admin, then the app. */
export async function devLogin(): Promise<void> {
  await post<Me>("/api/auth/dev", {});
  globalThis.location.assign("/");
}

/** Log out (ADR 0029 step 4.4): the server deletes the session and its cookie, then the login page shows. */
export async function logout(): Promise<void> {
  try {
    await post("/api/auth/logout");
  } finally {
    globalThis.location.assign(LOGIN_PATH);
  }
}
