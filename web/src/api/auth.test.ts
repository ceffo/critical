import { afterEach, describe, expect, it, vi } from "vitest";
import { ensureSession } from "./auth";

/** fetch answering each call with the next status, recording the calls. */
function answers(...statuses: number[]) {
  const fetch = vi.fn();
  for (const s of statuses)
    fetch.mockResolvedValueOnce(new Response("{}", { status: s }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

afterEach(() => vi.unstubAllGlobals());

describe("ensureSession", () => {
  it("does nothing more when the browser has a session", async () => {
    const fetch = answers(200);
    expect(await ensureSession()).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/me");
  });

  it("logs in through the dev login without a session", async () => {
    const fetch = answers(401, 200);
    expect(await ensureSession()).toBe(true);
    expect(fetch.mock.calls[1][0]).toBe("/api/auth/dev");
    expect(fetch.mock.calls[1][1]).toMatchObject({ method: "POST" });
  });

  it("goes to the Google login when the server has no dev login", async () => {
    answers(401, 404);
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    expect(await ensureSession()).toBe(false);
    expect(assign).toHaveBeenCalledWith("/api/auth/google/start");
  });

  it("lets the app load when the API is unreachable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")));
    expect(await ensureSession()).toBe(true);
  });
});
