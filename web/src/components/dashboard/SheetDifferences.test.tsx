import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { SheetDifference } from "@/types/character";
import { SheetDifferences } from "./Identity";

async function render(differences: SheetDifference[], lng: string) {
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
      <SheetDifferences differences={differences} />
    </I18nextProvider>,
  );
}

// ADR 0030 step 24: the server sends each difference as a code and two numbers; the card words
// them, the sheet's figure and the rules' one, in the player's language.
describe("SheetDifferences (ADR 0030 step 24)", () => {
  const diffs: SheetDifference[] = [
    { value: "thac0", sheet: 13, computed: 12 },
    { value: "save_spell", sheet: 9, computed: 7 },
  ];

  it("words every difference in English", async () => {
    const html = await render(diffs, "en");
    expect(html).toContain('role="status"');
    expect(html).toContain("The sheet differs from the rules");
    expect(html).toContain("THAC0: sheet 13, rules 12");
    expect(html).toContain("Save vs Spells: sheet 9, rules 7");
    expect(html).toContain("The app uses the rules&#x27; value.");
  });

  it("words every difference in French", async () => {
    const html = await render(diffs, "fr");
    expect(html).toContain("La feuille diffère des règles");
    expect(html).toContain("TAC0 : feuille 13, règles 12");
    expect(html).toContain("JS Sorts : feuille 9, règles 7");
  });
});
