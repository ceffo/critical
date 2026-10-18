import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { useSessionStore } from "@/stores/session";
import { ApiError, CHARACTER_HEADER, post, request } from "./http";

function respond(status: number, body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify(body), { status, statusText: "x" }),
      ),
  );
}

afterEach(async () => {
  vi.unstubAllGlobals();
  useSessionStore.getState().setMe(null);
  await i18n.changeLanguage("en");
});

describe("request errors", () => {
  it.each([
    ["en", "There is no initiative to reroll: start a combat first."],
    ["fr", "Il n'y a pas d'initiative à relancer : lancez d'abord un combat."],
  ])("words a known error code in %s", async (lng, want) => {
    await i18n.changeLanguage(lng);
    respond(409, {
      code: "no_initiative",
      error: "conflict: no initiative to reroll",
    });
    const err = await request("/api/x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).message).toBe(want);
    expect((err as ApiError).code).toBe("no_initiative");
    expect((err as ApiError).status).toBe(409);
  });

  it.each([
    ["en", "That spell is not in the catalogue."],
    ["fr", "Ce sort n'est pas dans le catalogue."],
  ])(
    "words a stale spell id's 404 in %s, which the details popup shows",
    async (lng, want) => {
      await i18n.changeLanguage(lng);
      respond(404, { code: "spell_not_found", error: "spell not found: gone" });
      const err = await request("/api/spells/gone").catch((e: unknown) => e);
      expect((err as ApiError).message).toBe(want);
      expect((err as ApiError).status).toBe(404);
    },
  );

  it.each([
    ["an unknown code", { code: "brand_new", error: "server words" }],
    ["a generic code", { code: "invalid_input", error: "server words" }],
    ["the internal code", { code: "internal", error: "server words" }],
    ["the bad_request code", { code: "bad_request", error: "server words" }],
    ["no code", { error: "server words" }],
  ])("falls back to the server message for %s", async (_name, body) => {
    respond(400, body);
    const err = await request("/api/x").catch((e: unknown) => e);
    expect((err as ApiError).message).toBe("server words");
  });
});

describe("an ended session", () => {
  it("shows the login page saying the session ended on a 401", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    respond(401, { code: "unauthenticated", error: "authentication required" });
    const err = await request("/api/character").catch((e: unknown) => e);
    expect((err as ApiError).status).toBe(401);
    expect(assign).toHaveBeenCalledWith("/login?error=session_expired");
  });

  it("leaves a 401 of the login endpoints to their caller", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    respond(401, { code: "unauthenticated", error: "authentication required" });
    await request("/api/auth/logout", { method: "POST" }).catch(
      () => undefined,
    );
    expect(assign).not.toHaveBeenCalled();
  });

  it("stays on the page for other errors", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    respond(409, { code: "no_initiative", error: "x" });
    await request("/api/x").catch(() => undefined);
    expect(assign).not.toHaveBeenCalled();
  });
});

describe("the character the page shows (ADR 0029 step 7.2)", () => {
  const me = (currentCharacterId: number | null) =>
    useSessionStore.getState().setMe({
      id: "u",
      email: "ann@example.com",
      role: "player",
      root: false,
      currentCharacterId,
    });
  const sentHeaders = () =>
    new Headers(
      (vi.mocked(fetch).mock.calls[0][1] as RequestInit | undefined)?.headers,
    );

  it("names it on every request, keeping the request's own headers", async () => {
    me(7);
    respond(200, {});
    await post("/api/hp/event", { delta: -1 });
    expect(sentHeaders().get(CHARACTER_HEADER)).toBe("7");
    expect(sentHeaders().get("Content-Type")).toBe("application/json");
  });

  it("names none when the user has no character yet", async () => {
    me(null);
    respond(200, {});
    await request("/api/characters");
    expect(sentHeaders().has(CHARACTER_HEADER)).toBe(false);
  });

  it("reloads the page when the character changed elsewhere", async () => {
    const reload = vi.fn();
    vi.stubGlobal("location", { reload, assign: vi.fn() });
    me(7);
    respond(409, {
      code: "character_changed",
      error: "current character changed: conflict",
    });
    const err = await request("/api/hp/event", { method: "POST" }).catch(
      (e: unknown) => e,
    );
    expect((err as ApiError).code).toBe("character_changed");
    expect(reload).toHaveBeenCalledOnce();
  });
});
