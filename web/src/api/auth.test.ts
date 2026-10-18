import { afterEach, describe, expect, it, vi } from "vitest";
import { loginOptions, logout, readSession } from "./auth";

/** fetch answering each call with the next status and body, recording the calls. */
function answers(...replies: [number, unknown][]) {
  const fetch = vi.fn();
  for (const [status, body] of replies)
    fetch.mockResolvedValueOnce(new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

afterEach(() => vi.unstubAllGlobals());

describe("readSession", () => {
  it("is the logged-in user", async () => {
    const me = {
      id: "u1",
      email: "ann@example.com",
      role: "admin",
      root: true,
    };
    const fetch = answers([200, me]);
    expect(await readSession()).toEqual({ kind: "in", me });
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/me");
  });

  it.each([["unauthenticated"], ["session_expired"]])(
    "is no session on a 401, with the server's code %s",
    async (code) => {
      answers([401, { code, error: "x" }]);
      expect(await readSession()).toEqual({ kind: "out", code });
    },
  );

  it("is no answer when the API is unreachable or fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")));
    expect(await readSession()).toEqual({ kind: "down" });
    answers([502, {}]);
    expect(await readSession()).toEqual({ kind: "down" });
  });
});

describe("loginOptions", () => {
  it("is the server's methods", async () => {
    answers([200, { google: false, devLogin: true }]);
    expect(await loginOptions()).toEqual({ google: false, devLogin: true });
  });

  it("offers Google when the API does not answer", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")));
    expect(await loginOptions()).toEqual({ google: true, devLogin: false });
  });
});

describe("logout", () => {
  it("ends the session on the server, then shows the login page", async () => {
    const fetch = answers([200, {}]);
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    await logout();
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/logout");
    expect(fetch.mock.calls[0][1]).toMatchObject({ method: "POST" });
    expect(assign).toHaveBeenCalledWith("/login");
  });

  it("with a dead session goes to the login page once, saying nothing", async () => {
    answers([
      401,
      { code: "unauthenticated", error: "authentication required" },
    ]);
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    await logout().catch(() => undefined);
    expect(assign).toHaveBeenCalledTimes(1);
    expect(assign).toHaveBeenCalledWith("/login");
  });
});
