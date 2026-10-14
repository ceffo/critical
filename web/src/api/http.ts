/** An API error carrying the server's message ({"error": "..."}) and HTTP status. */
export class ApiError extends Error {
  readonly status: number;
  /** Request field the message is about (a validation error), when the server names one. */
  readonly field?: string;
  constructor(message: string, status: number, field?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.field = field;
  }
}

/** Server message (and the field it is about, if any) from an error response: the JSON body, else the raw text. */
async function errorBody(
  res: Response,
): Promise<{ message: string; field?: string }> {
  const text = await res.text();
  try {
    const body = JSON.parse(text) as { error?: unknown; field?: unknown };
    if (typeof body.error === "string" && body.error) {
      return {
        message: body.error,
        field: typeof body.field === "string" ? body.field : undefined,
      };
    }
  } catch {
    // not JSON
  }
  return { message: text.trim() || `${res.status} ${res.statusText}` };
}

/** fetch + JSON decode; throws ApiError with the server's message on a non-2xx status. */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    const { message, field } = await errorBody(res);
    throw new ApiError(message, res.status, field);
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
