import i18n from "@/i18n";

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

/** fetch + JSON decode; throws ApiError with the localized or server message on a non-2xx status. */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    const { message, field, code } = await errorBody(res);
    throw new ApiError(message, res.status, field, code);
  }
  return res.json() as Promise<T>;
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
