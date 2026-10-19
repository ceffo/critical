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
  // The answer is the cast's logged payload (castsvc.CastBreakdown): the view reads its names.
  it("shows the spell, the item spent and the attack's d20 from the logged names", async () => {
    const html = await render({
      logEntryId: 1,
      action: "cast",
      spellId: "magic_missile",
      spell: "Magic Missile",
      level: 1,
      casterLevel: 1,
      kind: "damage",
      untracked: [],
      item: { itemId: "diamond", name: "Diamond", remaining: 3 },
      attack: {
        d20: 17,
        targetAc: 5,
        hit: true,
        effectiveThac0: 11,
        targetACNeeded: -6,
      },
    });
    expect(html).toContain("Magic Missile");
    expect(html).toContain('data-testid="cast-item"');
    expect(html).toContain("Diamond");
    expect(html).toMatch(/title="d20"[^>]*>17</);
  });

  it("shows the catalogue spell's class and text when the answer carries it", async () => {
    const html = await render({
      logEntryId: 1,
      action: "received",
      spellId: "sleep",
      spell: "Sleep",
      casterLevel: 5,
      catalogueSpell: {
        id: "sleep",
        name: { en: "Sleep" },
        class: "wizard",
        level: 1,
        description: { en: "Puts creatures to sleep." },
      } as never,
    });
    expect(html).toContain('data-testid="cast-spell-ident"');
    expect(html).toContain("Puts creatures to sleep.");
  });
});

// ADR 0030 step 14: a cast above the failure chance shows its d100 and resolves as usual.
describe("CastResultView spell failure", () => {
  it("shows the d100 of a cast that held", async () => {
    const html = await render({
      logEntryId: 2,
      action: "cast",
      spellId: "bless",
      spell: "Bless",
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
