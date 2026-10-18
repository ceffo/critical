import i18n from "@/i18n";
import { useDatabaseStore } from "@/stores/database";

/**
 * An API error carrying the HTTP status and a message: the player's language when the server sent a
 * known error code ({"code": "..."}), else the server's own text ({"error": "..."}).
 */
export class ApiError extends Error {
  readonly status: number;
  /** The server's stable error code (e.g. "no_initiative"), when it sent one. */
  readonly code?: string;
  /** Request field the message is about (a validation error), when the server names one. */
  readonly field?: string;
  constructor(message: string, status: number, field?: string, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.field = field;
    this.code = code;
  }
}

/** The localized message of an error code, or null when the web has no wording for it (the server text then stands). */
export function codeMessage(code: string | undefined): string | null {
  const key = `apiError.${code}`;
  return code && i18n.exists(key) ? i18n.t(key) : null;
}

/** Message (and the field and code it comes with, if any) from an error response: the JSON body, else the raw text. */
async function errorBody(
  res: Response,
): Promise<{ message: string; field?: string; code?: string }> {
  const text = await res.text();
  try {
    const body = JSON.parse(text) as {
      error?: unknown;
      field?: unknown;
      code?: unknown;
    };
    if (typeof body.error === "string" && body.error) {
      const code = typeof body.code === "string" ? body.code : undefined;
      return {
        message: codeMessage(code) ?? body.error,
        field: typeof body.field === "string" ? body.field : undefined,
        code,
      };
    }
  } catch {
    // not JSON
  }
  return { message: text.trim() || `${res.status} ${res.statusText}` };
}

/** The code of a request the database could not serve in time (HTTP 503). */
export const UNAVAILABLE = "database_unavailable";

/** Whether message is the wording of `database_unavailable` in any language (an error notice
 * keeps the text of the language it was raised in). */
export function isUnavailableMessage(message: string | null | undefined) {
  if (!message) return false;
  const key = `apiError.${UNAVAILABLE}`;
  return Object.keys(i18n.options.resources ?? {}).some(
    (lng) => i18n.t(key, { lng }) === message,
  );
}

/** How long a read waits before it asks again while the database is unavailable. */
export const RETRY_MS = 3000;

/** How many times a read asks before it gives up and throws (about a minute). */
export const MAX_READ_ATTEMPTS = 20;

/** What the probe reads while the database is unavailable: a small read that needs it. */
const PROBE_URL = "/api/character";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

function unavailableError(): ApiError {
  return new ApiError(
    codeMessage(UNAVAILABLE) ?? "database unavailable",
    503,
    undefined,
    UNAVAILABLE,
  );
}

let reload: (() => Promise<unknown>) | null = null;
let recovering = false;
let probing = false;

/**
 * Registers what reloads the whole sheet from the database: after a write whose outcome is
 * unknown, it runs once the database answers again, before writes are allowed again.
 */
export function setReload(fn: (() => Promise<unknown>) | null) {
  reload = fn;
}

/** The database did not answer: show it, and start the probe that notices when it is back. */
function markUnavailable(write: boolean) {
  useDatabaseStore.setState((s) => ({
    unavailable: true,
    uncertainWrite: s.uncertainWrite || write,
  }));
  if (!probing) void probe();
}

/** The database answered. After an uncertain write the sheet is reloaded first, so the player sees
 * whether it was saved before acting again. */
async function markAnswered() {
  const s = useDatabaseStore.getState();
  if (!s.unavailable || recovering) return;
  if (!s.uncertainWrite || !reload) {
    useDatabaseStore.setState({ unavailable: false, uncertainWrite: false });
    return;
  }
  recovering = true;
  try {
    await reload();
    useDatabaseStore.setState({ unavailable: false, uncertainWrite: false });
  } catch {
    // Still unavailable: the probe goes on.
  } finally {
    recovering = false;
  }
}

/** Reads PROBE_URL every RETRY_MS while the database is unavailable (a failed write leaves no read
 * waiting to notice it is back). */
async function probe() {
  probing = true;
  try {
    while (useDatabaseStore.getState().unavailable) {
      await sleep(RETRY_MS);
      if (recovering) continue;
      try {
        await request(PROBE_URL);
      } catch {
        // keep probing
      }
    }
  } finally {
    probing = false;
  }
}

const isRead = (init?: RequestInit) =>
  !init?.method || init.method === "GET" || init.method === "HEAD";

/**
 * fetch + JSON decode; throws ApiError with the localized or server message on a non-2xx status.
 *
 * While the database is unavailable (`database_unavailable`, ADR 0025 step 10.4) a read asks again
 * every RETRY_MS, up to MAX_READ_ATTEMPTS, so the panels wait instead of failing. A write is never
 * sent again by itself: it fails, and so does every write until the database answers (and, after a
 * write whose outcome is unknown, until the sheet is reloaded), so a second click cannot record the
 * same action twice.
 */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const read = isRead(init);
  if (!read && useDatabaseStore.getState().unavailable) {
    throw unavailableError();
  }
  for (let attempt = 1; ; attempt++) {
    const res = await fetch(url, init);
    if (res.ok) {
      void markAnswered();
      return res.json() as Promise<T>;
    }
    const { message, field, code } = await errorBody(res);
    const err = new ApiError(message, res.status, field, code);
    if (code !== UNAVAILABLE) {
      if (res.status < 500) void markAnswered();
      throw err;
    }
    markUnavailable(!read);
    if (!read || attempt >= MAX_READ_ATTEMPTS) throw err;
    await sleep(RETRY_MS);
  }
}

/** POST a JSON body (or nothing) and decode the JSON response. */
export function post<T>(url: string, body?: unknown): Promise<T> {
  return request<T>(url, {
    method: "POST",
    ...(body === undefined
      ? {}
      : {
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }),
  });
}

/** PUT a JSON body and decode the JSON response. */
export function put<T>(url: string, body: unknown): Promise<T> {
  return request<T>(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** PATCH a JSON body and decode the JSON response. */
export function patch<T>(url: string, body: unknown): Promise<T> {
  return request<T>(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** DELETE and decode the JSON response. */
export function del<T>(url: string): Promise<T> {
  return request<T>(url, { method: "DELETE" });
}
