import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ApiError, request } from "./http";

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

  it("stays on the page for other errors", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { assign });
    respond(409, { code: "no_initiative", error: "x" });
    await request("/api/x").catch(() => undefined);
    expect(assign).not.toHaveBeenCalled();
  });
});
