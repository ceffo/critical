import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import {
  ApiError,
  NETWORK_ERROR,
  REQUEST_FAILED,
  SERVER_ERROR,
  post,
  request,
} from "./http";

const answer = (status: number, body: string) =>
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response(body, { status }))),
  );

const fail = (run: () => Promise<unknown>) =>
  run().then(
    () => {
      throw new Error("expected a failure");
    },
    (e: unknown) => e as ApiError,
  );

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(async () => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  await i18n.changeLanguage("en");
});

describe("an error with no known code is worded, never raw", () => {
  it.each([
    ["500 with {}", 500, "{}"],
    ["500 with text", 500, "boom: nil pointer at foo.go:12"],
    ["500 with an uncoded JSON error", 500, '{"error":"boom"}'],
    ["500 with no body", 500, ""],
    ["502 with an HTML page", 502, "<html>Bad Gateway</html>"],
  ])("%s", async (_name, status, body) => {
    answer(status, body);
    const en = await fail(() => post("/api/hp/event", {}));
    expect(en.status).toBe(status);
    expect(en.code).toBe(SERVER_ERROR);
    expect(en.message).toBe(
      "Something went wrong on the server. Try again in a moment.",
    );
    await i18n.changeLanguage("fr");
    const fr = await fail(() => post("/api/hp/event", {}));
    expect(fr.message).toBe(
      "Une erreur est survenue sur le serveur. Réessayez dans un instant.",
    );
    expect(`${en.message}${fr.message}`).not.toMatch(/[{}<]|boom/);
  });

  it("a refusal with no readable text is worded too", async () => {
    answer(404, "404 page not found");
    const e = await fail(() => request("/api/nope"));
    expect(e.code).toBe(REQUEST_FAILED);
    expect(e.message).toBe(
      "The server refused this request. Reload the page and try again.",
    );
  });

  it("keeps the wording of a known code and the text of a refusal", async () => {
    answer(409, '{"error":"raw","code":"no_initiative"}');
    expect((await fail(() => request("/api/x"))).message).toBe(
      "There is no initiative to reroll: start a combat first.",
    );
    answer(400, '{"error":"formula is empty","field":"formula"}');
    const e = await fail(() => request("/api/x"));
    expect(e.message).toBe("formula is empty");
    expect(e.field).toBe("formula");
  });

  it("a request that got no answer says the server cannot be reached", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new TypeError("Failed to fetch"))),
    );
    const en = await fail(() => post("/api/hp/event", {}));
    expect(en.code).toBe(NETWORK_ERROR);
    expect(en.status).toBe(0);
    expect(en.message).toBe(
      "The server cannot be reached. Check your connection and try again.",
    );
    await i18n.changeLanguage("fr");
    expect((await fail(() => request("/api/x"))).message).toBe(
      "Le serveur est injoignable. Vérifiez votre connexion et réessayez.",
    );
  });
});
