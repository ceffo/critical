import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { CastResult } from "@/types/spells";
import { CastResultView } from "./CastResultView";

async function render(result: CastResult) {
  const inst = i18next.createInstance();
  await inst.init({
    lng: "fr",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <CastResultView result={result} />
    </I18nextProvider>,
  );
}

describe("CastResultView", () => {
  it("does not show the developer note of a spell effect", async () => {
    const html = await render({
      entryId: 1,
      source: "cast",
      spellId: "magic_missile",
      name: "Magic Missile",
      level: 1,
      casterLevel: 1,
      kind: "damage",
      note: "Only the damage is modelled, the rest is left to the table.",
      untracked: [],
    });
    expect(html).toContain("Magic Missile");
    expect(html).not.toContain("Only the damage is modelled");
  });
});

// ADR 0030 step 14: a cast above the failure chance shows its d100 and resolves as usual.
describe("CastResultView spell failure", () => {
  it("shows the d100 of a cast that held", async () => {
    const html = await render({
      entryId: 2,
      source: "cast",
      spellId: "bless",
      name: "Bless",
      level: 1,
      casterLevel: 1,
      untracked: [],
      failure: { d100: 55, chance: 20, failed: false },
    });
    expect(html).toContain('data-testid="cast-failure"');
    expect(html).toContain("55");
    expect(html).toContain("Bless");
  });
});
