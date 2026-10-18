import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { LevelUp } from "@/types/character";
import { LevelUpPrompt } from "./LevelUpPrompt";

async function render(levelUp: LevelUp, lng: string, level = 15) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <LevelUpPrompt
        levelUp={levelUp}
        level={level}
        busy={false}
        onRoll={() => Promise.resolve(null)}
      />
    </I18nextProvider>,
  );
}

// ADR 0030 step 7: the server sends the pending levels (lowest first) with their die or fixed
// gain; the card prompts for the first one, French first.
describe("LevelUpPrompt (ADR 0030 step 7)", () => {
  it("prompts for the lowest pending level's hit die", async () => {
    const lu: LevelUp = {
      hpLevel: 8,
      pending: [
        { level: 9, die: 6, fixedGain: 0 },
        { level: 10, die: 6, fixedGain: 0 },
      ],
      hpAboveLevel: false,
    };
    const fr = await render(lu, "fr", 10);
    expect(fr).toContain("Niveau 9 atteint");
    expect(fr).toContain("Dé de vie d6, plus l&#x27;ajustement de CON");
    expect(fr).toContain(">Lancer<");
    expect(fr).toContain(">Saisir<");
    expect(fr).toContain("+1 autre niveau à lancer ensuite");
    const en = await render(lu, "en", 10);
    expect(en).toContain("Level 9 reached");
    expect(en).toContain(">Roll<");
  });

  it("offers the fixed gain above the name level, with no die to enter", async () => {
    const lu: LevelUp = {
      hpLevel: 15,
      pending: [{ level: 16, die: 0, fixedGain: 2 }],
      hpAboveLevel: false,
    };
    const fr = await render(lu, "fr", 16);
    expect(fr).toContain("Niveau 16 atteint");
    expect(fr).toContain("Au-delà du niveau de nom : +2 PV max, sans dé");
    expect(fr).toContain(">Ajouter<");
    expect(fr).not.toContain(">Saisir<");
    expect(fr).not.toContain("autre niveau");
  });

  it("shows nothing when no level is owed", async () => {
    expect(
      await render({ hpLevel: 15, pending: [], hpAboveLevel: false }, "fr"),
    ).toBe("");
  });

  it("warns when the XP gives a level under the max HP's (step 7.4)", async () => {
    const lu: LevelUp = { hpLevel: 16, pending: [], hpAboveLevel: true };
    expect(await render(lu, "fr")).toContain(
      "PV max lancés pour le niveau 16, l&#x27;XP donne le niveau 15",
    );
    expect(await render(lu, "en")).toContain(
      "Max HP rolled for level 16, XP gives level 15",
    );
  });
});
