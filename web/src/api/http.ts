/** An API error carrying the server's message ({"error": "..."}) and HTTP status. */
export class ApiError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** Server message from an error response: the `error` field of a JSON body, else the raw text. */
async function errorMessage(res: Response): Promise<string> {
  const text = await res.text();
  try {
    const body = JSON.parse(text) as { error?: unknown };
    if (typeof body.error === "string" && body.error) return body.error;
  } catch {
    // not JSON
  }
  return text.trim() || `${res.status} ${res.statusText}`;
}

/** fetch + JSON decode; throws ApiError with the server's message on a non-2xx status. */
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) throw new ApiError(await errorMessage(res), res.status);
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
