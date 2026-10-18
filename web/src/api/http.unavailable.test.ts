import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { useDatabaseStore } from "@/stores/database";
import {
  ApiError,
  MAX_READ_ATTEMPTS,
  RETRY_MS,
  UNAVAILABLE,
  post,
  request,
  setReload,
} from "./http";

const unavailable = () =>
  new Response(
    JSON.stringify({ code: UNAVAILABLE, error: "database unavailable" }),
    { status: 503 },
  );
const ok = (body: unknown = { ok: true }) =>
  new Response(JSON.stringify(body), { status: 200 });

/** fetch answering each call to a URL with that URL's next responder (the last one repeats);
 * other URLs (the probe's) get `rest`. */
function serve(
  responders: Record<string, (() => Response)[]>,
  rest: () => Response = unavailable,
) {
  const f = vi.fn((url: string) => {
    const q = responders[url];
    if (!q) return Promise.resolve(rest());
    const r = q.length > 1 ? q.shift()! : q[0];
    return Promise.resolve(r());
  });
  vi.stubGlobal("fetch", f);
  return f;
}

const state = () => useDatabaseStore.getState();

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(async () => {
  // The database answers: every probe loop ends.
  serve({}, () => ok());
  setReload(null);
  useDatabaseStore.setState({ unavailable: false, uncertainWrite: false });
  await vi.advanceTimersByTimeAsync(RETRY_MS * 2);
  vi.useRealTimers();
  vi.unstubAllGlobals();
  await i18n.changeLanguage("en");
});

describe("database unavailable (ADR 0025 step 10.4)", () => {
  it("retries a read every RETRY_MS, shows the state meanwhile, and clears it once answered", async () => {
    const f = serve({
      "/api/hp": [unavailable, unavailable, () => ok({ hp: 7 })],
    });
    const got = request<{ hp: number }>("/api/hp");
    await vi.advanceTimersByTimeAsync(0);
    expect(state().unavailable).toBe(true);
    expect(state().uncertainWrite).toBe(false);
    await vi.advanceTimersByTimeAsync(RETRY_MS * 2);
    await expect(got).resolves.toEqual({ hp: 7 });
    expect(f.mock.calls.filter(([u]) => u === "/api/hp")).toHaveLength(3);
    expect(state().unavailable).toBe(false);
  });

  it("gives a read up after MAX_READ_ATTEMPTS, with the localized message", async () => {
    await i18n.changeLanguage("fr");
    serve({});
    const got = request("/api/hp").catch((e: unknown) => e);
    await vi.advanceTimersByTimeAsync(RETRY_MS * MAX_READ_ATTEMPTS);
    const err = (await got) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(503);
    expect(err.code).toBe(UNAVAILABLE);
    expect(err.message).toBe(
      "La base de données ne répond pas. Réessayez dans un instant.",
    );
  });

  it("never sends a write twice, and refuses the next write until the sheet is reloaded", async () => {
    const f = serve({});
    const reload = vi.fn(() => Promise.resolve());
    setReload(reload);

    const first = await post("/api/hp/event", { delta: -3 }).catch(
      (e: unknown) => e,
    );
    expect((first as ApiError).code).toBe(UNAVAILABLE);
    expect(f).toHaveBeenCalledTimes(1);
    expect(state()).toMatchObject({ unavailable: true, uncertainWrite: true });

    // A second click while unavailable is refused without reaching the server.
    const second = await post("/api/hp/event", { delta: -3 }).catch(
      (e: unknown) => e,
    );
    expect((second as ApiError).code).toBe(UNAVAILABLE);
    expect(f.mock.calls.filter(([u]) => u === "/api/hp/event")).toHaveLength(1);

    // The database answers the probe: the sheet is reloaded, then writes are allowed.
    serve({}, () => ok());
    await vi.advanceTimersByTimeAsync(RETRY_MS);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(state()).toMatchObject({
      unavailable: false,
      uncertainWrite: false,
    });
    await expect(post("/api/hp/event", { delta: -3 })).resolves.toEqual({
      ok: true,
    });
  });

  it("keeps writes refused while the reload after an uncertain write fails", async () => {
    serve({});
    let reloads = 0;
    setReload(() => {
      reloads++;
      return reloads === 1
        ? Promise.reject(new Error("down"))
        : Promise.resolve();
    });
    await post("/api/xp", {}).catch(() => undefined);

    serve({}, () => ok());
    await vi.advanceTimersByTimeAsync(RETRY_MS);
    expect(reloads).toBe(1);
    expect(state().unavailable).toBe(true);
    await vi.advanceTimersByTimeAsync(RETRY_MS);
    expect(reloads).toBe(2);
    expect(state().unavailable).toBe(false);
  });
});
