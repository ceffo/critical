/**
 * Makes sure the browser has a session before the app loads (ADR 0029). The server answers 401 to every
 * data route without one. Until the login page exists, a server in dev login mode (no OAuth settings:
 * `just dev`, e2e) is logged into as its root admin, and a server with Google login sends the browser to it.
 * Resolves false when the page is navigating away.
 */
export async function ensureSession(): Promise<boolean> {
  try {
    return await signIn();
  } catch {
    return true; // the API is unreachable: the app loads and shows it
  }
}

async function signIn(): Promise<boolean> {
  const me = await fetch("/api/auth/me");
  if (me.status !== 401) return true; // signed in, or an error the app shows itself
  const dev = await fetch("/api/auth/dev", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
  if (dev.ok) return true;
  globalThis.location.assign("/api/auth/google/start");
  return false;
}
