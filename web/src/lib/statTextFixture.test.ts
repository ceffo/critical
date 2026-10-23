// Every spell of the API's fixture catalogue, its six stats worded in English and French
// (ADR 0021 step 26.4: the words come from the typed stat and its label, so the texts must not
// change). The snapshot is the table; `__fixtures__/spellStats.json` is `api/cmd/spellstats` run on
// `api/testdata/catalogue.json`. Regenerate both with `vitest -u` after a deliberate change.
import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { statLocale, statText, type StatField } from "./statText";
import type { Spell } from "@/types/spells";
import fixture from "./__fixtures__/spellStats.json";

const FIELDS: StatField[] = [
  "casting_time",
  "range",
  "duration",
  "area_of_effect",
  "components",
  "saving_throw",
];

type FixtureSpell = { id: string; stats: Spell["stats"] } & Record<
  StatField,
  string
>;

async function locale(lng: "en" | "fr") {
  const i = i18next.createInstance();
  await i.init({
    lng,
    resources,
    defaultNS: "common",
    interpolation: { escapeValue: false },
  });
  return statLocale(i.t, lng);
}

describe("statText (fixture spells)", () => {
  it("words every fixture spell's stats as before, en and fr", async () => {
    const rows: string[] = [];
    for (const lng of ["en", "fr"] as const) {
      const loc = await locale(lng);
      for (const s of fixture as unknown as FixtureSpell[]) {
        const spell = {
          stats: s.stats,
          ...Object.fromEntries(FIELDS.map((f) => [f, { en: s[f] }])),
        } as unknown as Spell;
        for (const f of FIELDS) {
          const { text } = statText(loc, spell, f);
          rows.push(`${lng}\t${s.id}\t${f}\t${text}`);
        }
      }
    }
    await expect(rows.join("\n") + "\n").toMatchFileSnapshot(
      "./__fixtures__/spellStatTexts.snap.txt",
    );
  });
});
