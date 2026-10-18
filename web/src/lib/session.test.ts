import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { LOGIN_ERRORS, loginErrorKey, startFor } from "./session";

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("login page refusal wording", () => {
  // Not invited, disabled and an ended session each have their own message, in both languages.
  it.each([
    [
      "en",
      "not_invited",
      "This Google account is not invited. Ask an admin for an invite, then log in again.",
    ],
    [
      "fr",
      "not_invited",
      "Ce compte Google n'est pas invité. Demandez une invitation à un administrateur, puis réessayez.",
    ],
    [
      "en",
      "user_disabled",
      "This account is disabled. Ask an admin to enable it again.",
    ],
    [
      "fr",
      "user_disabled",
      "Ce compte est désactivé. Demandez à un administrateur de le réactiver.",
    ],
    ["en", "session_expired", "Your session has ended. Log in again."],
    ["fr", "session_expired", "Votre session a pris fin. Reconnectez-vous."],
    ["en", "something_new", "The login did not complete. Try again."],
    ["fr", "something_new", "La connexion n'a pas abouti. Réessayez."],
  ])("%s: %s", async (lng, code, want) => {
    await i18n.changeLanguage(lng);
    expect(i18n.t(loginErrorKey(code)!)).toBe(want);
  });

  it("gives each refusal its own message in each language", async () => {
    for (const lng of ["en", "fr"]) {
      await i18n.changeLanguage(lng);
      const texts = LOGIN_ERRORS.map((c) => i18n.t(loginErrorKey(c)!));
      for (const c of LOGIN_ERRORS)
        expect(i18n.exists(`login.error.${c}`, { lng })).toBe(true);
      expect(new Set(texts).size).toBe(texts.length);
    }
  });

  it("shows no message without a code", () => {
    expect(loginErrorKey(null)).toBeNull();
    expect(loginErrorKey("")).toBeNull();
  });
});

describe("startFor", () => {
  const me = {
    kind: "in" as const,
    me: {
      id: "u",
      email: "a@b.c",
      role: "player" as const,
      root: false,
      currentCharacterId: 1,
    },
  };

  it("shows the app with a session", () => {
    expect(startFor("/", "", me)).toEqual({ show: "app" });
    expect(startFor("/spells/42", "", me)).toEqual({ show: "app" });
  });

  it("shows the admin section to an admin only", () => {
    const admin = {
      kind: "in" as const,
      me: { ...me.me, role: "admin" as const },
    };
    expect(startFor("/admin", "", admin)).toEqual({ show: "admin" });
    expect(startFor("/admin", "", me)).toEqual({ go: "/" });
    expect(startFor("/admin", "", { kind: "out" })).toEqual({ go: "/login" });
  });

  it("sends a logged-in user from the login page to the app", () => {
    expect(startFor("/login", "?error=not_invited", me)).toEqual({ go: "/" });
  });

  it("sends a page without a session to the login page, saying when the session ended", () => {
    expect(startFor("/", "", { kind: "out", code: "unauthenticated" })).toEqual(
      { go: "/login" },
    );
    expect(startFor("/", "", { kind: "out", code: "session_expired" })).toEqual(
      { go: "/login?error=session_expired" },
    );
  });

  it("shows the login page with the refusal of its address, else the session's end", () => {
    expect(startFor("/login", "?error=user_disabled", { kind: "out" })).toEqual(
      { show: "login", error: "user_disabled" },
    );
    expect(
      startFor("/login", "", { kind: "out", code: "session_expired" }),
    ).toEqual({ show: "login", error: "session_expired" });
    expect(
      startFor("/login", "", { kind: "out", code: "unauthenticated" }),
    ).toEqual({ show: "login", error: null });
  });

  it("loads the app or the page when the API does not answer", () => {
    expect(startFor("/", "", { kind: "down" })).toEqual({ show: "app" });
    expect(startFor("/login", "", { kind: "down" })).toEqual({
      show: "login",
      error: null,
    });
  });
});
