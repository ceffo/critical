import { describe, expect, it, vi, afterEach } from "vitest";
import i18n from "@/i18n";
import { createInvite } from "@/api/admin";
import { defaultInviteLanguage, invitePayload, mailHintKey } from "./invites";

afterEach(async () => {
  vi.unstubAllGlobals();
  await i18n.changeLanguage("en");
});

describe("the invite form's mail language", () => {
  it.each([
    ["fr", "fr"],
    ["fr-FR", "fr"],
    ["en", "en"],
    ["en-GB", "en"],
    ["de", "en"],
    ["", "en"],
  ])("starts on the UI language %s as %s", (ui, want) => {
    expect(defaultInviteLanguage(ui)).toBe(want);
  });

  it("builds the payload with the trimmed email and the chosen language", () => {
    expect(invitePayload("  Cid@Example.com ", "fr")).toEqual({
      email: "Cid@Example.com",
      language: "fr",
    });
  });

  it("sends the language with the invite", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(JSON.stringify({ id: "i1", mail: "sent" }), {
          status: 201,
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const p = invitePayload("a@b.c", "fr");
    await createInvite(p.email, p.language);
    const calls = fetchMock.mock.calls as unknown as [string, RequestInit][];
    expect(JSON.parse(calls[0][1].body as string)).toEqual({
      email: "a@b.c",
      language: "fr",
    });
  });
});

describe("the mail outcome hint", () => {
  it.each([
    ["sent", "en", "Invitation sent to cid@example.com."],
    [
      "failed",
      "en",
      "The email to cid@example.com could not be sent. Copy the link and send it yourself.",
    ],
    [
      "not_configured",
      "en",
      "Email is not set up on this server. Copy the link and send it yourself.",
    ],
    ["sent", "fr", "Invitation envoyée à cid@example.com."],
    [
      "failed",
      "fr",
      "L'e-mail à cid@example.com n'a pas pu être envoyé. Copiez le lien et envoyez-le vous-même.",
    ],
    [
      "not_configured",
      "fr",
      "L'envoi d'e-mails n'est pas configuré sur ce serveur. Copiez le lien et envoyez-le vous-même.",
    ],
  ] as const)("%s in %s", async (outcome, lang, want) => {
    await i18n.changeLanguage(lang);
    expect(i18n.t(mailHintKey(outcome), { email: "cid@example.com" })).toBe(
      want,
    );
  });
});
