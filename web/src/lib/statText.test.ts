import { beforeAll, describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  METRIC,
  areaText,
  castingTimeText,
  componentsText,
  durationText,
  formatNumber,
  formatValue,
  nounText,
  quantityText,
  rangeText,
  saveText,
  statLocale,
  statText,
  unitText,
  type StatLocale,
} from "./statText";
import type { Spell, SpellQuantity, SpellStats } from "@/types/spells";

let fr: StatLocale;
let en: StatLocale;
beforeAll(async () => {
  for (const lng of ["en", "fr"] as const) {
    const i = i18next.createInstance();
    await i.init({
      lng,
      resources,
      defaultNS: "common",
      interpolation: { escapeValue: false },
    });
    if (lng === "fr") fr = statLocale(i.t, "fr");
    else en = statLocale(i.t, "en");
  }
});

/** One-term quantity. */
const qty = (
  count: number,
  unit?: string,
  more: Partial<SpellQuantity["terms"][0]> = {},
): SpellQuantity => ({ terms: [{ count, unit, ...more }] });

// The units spellstats produces (its unit table, "sq"/"cu" areas and volumes, and "year", which it
// keeps), plus the metric units they convert to.
const BOOK_UNITS = [
  "segment",
  "round",
  "turn",
  "hour",
  "day",
  "week",
  "year",
  "inch",
  "foot",
  "yard",
  "mile",
  "sq inch",
  "sq foot",
  "sq yard",
  "sq mile",
  "cu inch",
  "cu foot",
  "cu yard",
];
const METRIC_UNITS = [...new Set(Object.values(METRIC).map(([, u]) => u))];
// The shapes spellstats' area parser reads (shapeRe in area.go).
const SHAPES = [
  "radius",
  "diameter",
  "cone",
  "cube",
  "square",
  "sphere",
  "globe",
  "cloud",
  "column",
  "spray",
  "hemisphere",
  "volume",
  "wall",
];

describe("statLocale", () => {
  it("is metric in French, imperial in English", () => {
    expect(fr.metric).toBe(true);
    expect(en.metric).toBe(false);
  });
});

describe("unitText", () => {
  it("has a word for every book and metric unit, in both languages", () => {
    for (const loc of [en, fr])
      for (const u of [...BOOK_UNITS, ...METRIC_UNITS])
        expect(unitText(loc, u, 2), `${loc.lang} ${u}`).toBeTruthy();
  });

  it("agrees in number in French, keeps the short labels in English", () => {
    expect(unitText(fr, "round", 1)).toBe("round");
    expect(unitText(fr, "round", 3)).toBe("rounds");
    expect(unitText(fr, "turn", 2)).toBe("tours");
    expect(unitText(fr, "hour", 1)).toBe("heure");
    expect(unitText(fr, "sq m", 4)).toBe("m²");
    expect(unitText(fr, "cu m", 4)).toBe("m³");
    expect(unitText(en, "round", 3)).toBe("rd");
    expect(unitText(en, "sq foot", 3)).toBe("sq ft");
  });

  it("is null for a word that is no unit", () => {
    expect(unitText(fr, "usage", 1)).toBeNull();
  });
});

describe("nounText", () => {
  const NOUNS = Object.keys(
    (resources.en.common.lookup as { nouns: Record<string, string> }).nouns,
  )
    .filter((k) => k.endsWith("_one"))
    .map((k) => k.slice(0, -4).replace(/_/g, " "));

  it("has a singular and a plural for every listed noun, in both languages", () => {
    expect(NOUNS.length).toBeGreaterThan(20);
    for (const loc of [en, fr])
      for (const n of NOUNS) {
        expect(nounText(loc, n, 1), `${loc.lang} ${n}`).toBeTruthy();
        expect(nounText(loc, n, 5), `${loc.lang} ${n}`).toBeTruthy();
      }
  });

  it("reads the book's plural as its noun", () => {
    expect(nounText(fr, "creature", 1)).toBe("créature");
    expect(nounText(fr, "creatures", 3)).toBe("créatures");
    expect(nounText(fr, "persons", 2)).toBe("personnes");
    expect(nounText(fr, "animal", 2)).toBe("animaux");
    expect(nounText(en, "persons", 2)).toBe("persons");
  });

  it("is null for an unlisted noun", () => {
    expect(nounText(fr, "x 20 x", 1)).toBeNull();
  });
});

describe("formatValue (the value at the caster level)", () => {
  it("converts lengths to metres in French as the French books do: 1 yd = 1 m, 1 ft = 0.3 m", () => {
    expect(formatValue(fr, { amount: 210, unit: "yard" })).toBe("210 m");
    expect(formatValue(fr, { amount: 30, unit: "foot" })).toBe("9 m");
    expect(formatValue(fr, { amount: 15, unit: "foot" })).toBe("4,5 m");
    expect(formatValue(fr, { amount: 6, unit: "inch" })).toBe("15 cm");
    expect(formatValue(fr, { amount: 1, unit: "mile" })).toBe("1,5 km");
    expect(formatValue(fr, { amount: 100, unit: "sq foot" })).toBe("9 m²");
    expect(formatValue(fr, { amount: 10, unit: "sq yard" })).toBe("10 m²");
    expect(formatValue(fr, { amount: 4, unit: "sq mile" })).toBe("9 km²");
    expect(formatValue(fr, { amount: 1, unit: "cu yard" })).toBe("1 m³");
    expect(formatValue(fr, { amount: 1, unit: "cu foot" })).toBe("27 litres");
    expect(formatValue(fr, { amount: 1000, unit: "cu foot" })).toBe(
      "27000 litres",
    );
  });

  it("does not round a converted length, as the French books do not", () => {
    // Level-scaled values the French Tome of Magic prints: 15 x 5 ft is "22,5 mètres", 13 x 20 ft
    // "78 mètres", 13 x 2 ft "7,8 m".
    expect(formatValue(fr, { amount: 75, unit: "foot" })).toBe("22,5 m");
    expect(formatValue(fr, { amount: 260, unit: "foot" })).toBe("78 m");
    expect(formatValue(fr, { amount: 26, unit: "foot" })).toBe("7,8 m");
    expect(formatValue(fr, { amount: 930, unit: "foot" })).toBe("279 m");
    expect(formatValue(fr, { amount: 7, unit: "mile" })).toBe("10,5 km");
  });

  it("writes a length under a metre in centimetres, as the French books do", () => {
    expect(formatValue(fr, { amount: 1, unit: "foot" })).toBe("30 cm");
    expect(formatValue(fr, { amount: 3, unit: "foot" })).toBe("90 cm");
    expect(formatValue(fr, { amount: 5, unit: "foot" })).toBe("1,5 m");
    expect(formatValue(fr, { amount: 1, upper: 3, unit: "foot" })).toBe(
      "30-90 cm",
    );
    // A range is in one unit: the larger number's.
    expect(formatValue(fr, { amount: 2, upper: 5, unit: "foot" })).toBe(
      "0,6-1,5 m",
    );
    expect(
      quantityText(fr, qty(1, "foot", { per_level: true, step: 1 })),
    ).toBe("30 cm/niveau");
    expect(
      quantityText(fr, {
        terms: [{ count: 1, unit: "foot", per_level: true, step: 1 }],
        max: 2,
        max_unit: "foot",
      }),
    ).toBe("30 cm/niveau, max. 60 cm");
    // Areas, volumes and imperial units are left as they are.
    expect(formatValue(fr, { amount: 1, unit: "sq foot" })).toBe("0,09 m²");
    expect(formatValue(fr, { amount: 0.5, unit: "yard" })).toBe("50 cm");
    expect(formatValue(fr, { amount: 0, unit: "foot" })).toBe("0 m");
    expect(formatValue(en, { amount: 1, unit: "foot" })).toBe("1 ft");
  });

  it("keeps a volume's significant digits, not a fixed number of decimals", () => {
    // "1 cu. ft./level" at level 20: 20 cu ft is 566 litres; the books' 27 litres a foot gives 540.
    const at20 = formatValue(fr, { amount: 20, unit: "cu foot" });
    expect(at20).toBe("540 litres");
    expect(Math.abs(540 / (20 * 28.3168) - 1)).toBeLessThan(0.05);
    // "10 cu. in./level" at level 1: 156 cm³, not "156,25".
    expect(formatValue(fr, { amount: 10, unit: "cu inch" })).toBe("156 cm³");
    expect(formatValue(fr, { amount: 1234, unit: "cu foot" })).toBe(
      "33318 litres",
    );
    expect(formatNumber(fr, 0.015625, "cu m")).toBe("0,0156");
    expect(
      quantityText(fr, qty(1, "cu foot", { per_level: true, step: 1 })),
    ).toBe("27 litres/niveau");
  });

  it("keeps times, words their units in French, and leaves dice alone", () => {
    expect(formatValue(fr, { amount: 15, unit: "round" })).toBe("15 rounds");
    expect(formatValue(fr, { amount: 1, unit: "turn" })).toBe("1 tour");
    expect(formatValue(fr, { amount: 8, dice: "1d8", unit: "round" })).toBe(
      "8 + 1d8 rounds",
    );
    expect(formatValue(fr, { amount: 0, dice: "1d4", unit: "hour" })).toBe(
      "1d4 heures",
    );
    expect(formatValue(fr, { amount: 1.5, unit: "round" })).toBe("1,5 round");
    expect(formatValue(fr, { amount: 3, upper: 4, unit: "turn" })).toBe(
      "3-4 tours",
    );
  });

  it("counts targets in the language's nouns", () => {
    expect(formatValue(fr, { amount: 15 }, "creature")).toBe("15 créatures");
    expect(formatValue(fr, { amount: 1 }, "creature")).toBe("1 créature");
    expect(formatValue(fr, { amount: 0, dice: "1d4" }, "persons")).toBe(
      "1d4 personnes",
    );
    expect(formatValue(en, { amount: 15 }, "creature")).toBe("15 creatures");
  });

  it("is the book's units in English, as before", () => {
    expect(formatValue(en, { amount: 210, unit: "yard" })).toBe("210 yd");
    expect(formatValue(en, { amount: 1000, unit: "sq foot" })).toBe(
      "1000 sq ft",
    );
    expect(formatValue(en, { amount: 8, dice: "1d8", unit: "round" })).toBe(
      "8 + 1d8 rd",
    );
  });
});

describe("quantityText (the book's formula)", () => {
  it("words each term, per level and per n levels, metric in French", () => {
    const mm: SpellQuantity = {
      terms: [
        { count: 60, unit: "yard" },
        { count: 10, unit: "yard", per_level: true, step: 1 },
      ],
    };
    expect(quantityText(fr, mm)).toBe("60 m + 10 m/niveau");
    expect(quantityText(en, mm)).toBe("60 yd + 10 yd/level");
    expect(
      quantityText(fr, qty(1, "round", { per_level: true, step: 2 })),
    ).toBe("1 round/2 niveaux");
    expect(
      quantityText(fr, qty(2, "round", { per_level: true, step: 1 })),
    ).toBe("2 rounds/niveau");
  });

  it("words dice, ranges, 'up to' and a cap", () => {
    expect(quantityText(fr, qty(1, "turn", { dice: "1d4" }))).toBe(
      "1d4+1 tours",
    );
    expect(quantityText(fr, qty(0, "hour", { dice: "2d12" }))).toBe(
      "2d12 heures",
    );
    expect(quantityText(fr, qty(1, "day", { upper: 4 }))).toBe("1-4 jours");
    expect(quantityText(fr, { ...qty(4, "hour"), up_to: true })).toBe(
      "jusqu'à 4 heures",
    );
    expect(
      quantityText(fr, {
        terms: [{ count: 10, unit: "yard", per_level: true, step: 1 }],
        max: 100,
        max_unit: "yard",
      }),
    ).toBe("10 m/niveau, max. 100 m");
  });

  it("counts a noun", () => {
    expect(
      quantityText(
        fr,
        qty(1, undefined, { per_level: true, step: 1 }),
        "creature",
      ),
    ).toBe("1 créature/niveau");
    expect(
      quantityText(fr, qty(0, undefined, { dice: "2d4" }), "creatures"),
    ).toBe("2d4 créatures");
  });

  it("is null for a word the language lacks or a non-level divisor", () => {
    expect(quantityText(fr, qty(1, "rum"))).toBeNull();
    expect(quantityText(fr, qty(1, "round", { per: "recipient" }))).toBeNull();
    expect(quantityText(fr, qty(1), "x 20 x")).toBeNull();
    expect(quantityText(fr, { terms: [] })).toBeNull();
  });
});

const none: SpellQuantity = { terms: [] };

describe("castingTimeText", () => {
  it("words a time, a book 'Special' and a special with a number", () => {
    expect(
      castingTimeText(fr, { raw: "3", kind: "timed", time: qty(3, "segment") }),
    ).toBe("3 segments");
    expect(
      castingTimeText(fr, {
        raw: "1 rd.",
        kind: "timed",
        time: qty(1, "round"),
      }),
    ).toBe("1 round");
    expect(
      castingTimeText(fr, { raw: "Special", kind: "special", time: none }),
    ).toBe("Spécial");
    expect(
      castingTimeText(fr, {
        raw: "Special (8 turns)",
        kind: "special",
        time: qty(8, "turn"),
      }),
    ).toBe("Spécial (8 tours)");
  });

  it("is null for an untyped text", () => {
    expect(
      castingTimeText(fr, { raw: "Special + 1", kind: "special", time: none }),
    ).toBeNull();
  });
});

describe("rangeText", () => {
  const r = (
    kind: SpellStats["range"]["kind"],
    raw: string,
    distance = none,
    radius?: boolean,
  ): SpellStats["range"] => ({ raw, kind, distance, radius });

  it("words every kind", () => {
    expect(rangeText(fr, r("self", "0"))).toBe("0");
    expect(rangeText(fr, r("touch", "Touch"))).toBe("Contact");
    expect(rangeText(fr, r("unlimited", "Unlimited"))).toBe("Illimitée");
    expect(rangeText(fr, r("distance", "30 yds.", qty(30, "yard")))).toBe(
      "30 m",
    );
    expect(
      rangeText(fr, r("distance", "10-ft. radius", qty(10, "foot"), true)),
    ).toBe("rayon de 3 m");
    expect(rangeText(fr, r("special", "Special"))).toBe("Spéciale");
    expect(rangeText(fr, r("special", "Change"))).toBeNull();
  });
});

describe("durationText", () => {
  const d = (
    kind: SpellStats["duration"]["kind"],
    raw: string,
    more: Partial<SpellStats["duration"]> = {},
  ): SpellStats["duration"] => ({ raw, kind, time: none, ...more });

  it("words every kind", () => {
    expect(
      durationText(
        fr,
        d("timed", "1 turn + 1 rd./level", {
          time: {
            terms: [
              { count: 1, unit: "turn" },
              { count: 1, unit: "round", per_level: true, step: 1 },
            ],
          },
        }),
      ),
    ).toBe("1 tour + 1 round/niveau");
    expect(durationText(fr, d("instantaneous", "Instantaneous"))).toBe(
      "Instantanée",
    );
    expect(durationText(fr, d("concentration", "Concentration"))).toBe(
      "Concentration",
    );
    expect(durationText(fr, d("permanent", "Permanent"))).toBe("Permanente");
    expect(
      durationText(
        fr,
        d("permanent", "Permanent until dispelled", {
          condition: "dispelled",
        }),
      ),
    ).toBe("Permanente jusqu'à dissipation");
    expect(
      durationText(
        fr,
        d("until", "Until discharged", { condition: "discharged" }),
      ),
    ).toBe("Jusqu'à décharge");
    expect(durationText(fr, d("special", "Special"))).toBe("Spéciale");
    expect(
      durationText(
        fr,
        d("special", "Special (up to three rounds)", {
          time: { ...qty(3, "round"), up_to: true },
        }),
      ),
    ).toBe("Spéciale (jusqu'à 3 rounds)");
  });

  it("has words for every condition it lists, in both languages; null for another", () => {
    const conds = Object.keys(
      (resources.en.common.lookup as { conditions: Record<string, string> })
        .conditions,
    );
    for (const loc of [en, fr])
      for (const c of conds)
        expect(
          durationText(
            loc,
            d("until", "", { condition: c.replace(/_/g, " ") }),
          ),
        ).toBeTruthy();
    expect(
      durationText(
        fr,
        d("until", "Until the moon sets", { condition: "the moon sets" }),
      ),
    ).toBeNull();
    expect(durationText(fr, d("special", "Time of chanting"))).toBeNull();
  });
});

describe("areaText", () => {
  const a = (
    kind: SpellStats["area"]["kind"],
    raw: string,
    more: Partial<SpellStats["area"]> = {},
  ): SpellStats["area"] => ({ raw, kind, ...more });

  it("has a word for every shape the parser reads, in both languages", () => {
    for (const loc of [en, fr])
      for (const shape of SHAPES)
        expect(
          areaText(loc, a("area", "", { shape, dims: [qty(10, "foot")] })),
          `${loc.lang} ${shape}`,
        ).toMatch(/\S/);
  });

  it("words extents, shapes, counts and both", () => {
    expect(
      areaText(
        fr,
        a("area", "20-ft. radius", {
          shape: "radius",
          dims: [qty(20, "foot")],
        }),
      ),
    ).toBe("rayon de 6 m");
    expect(
      areaText(
        en,
        a("area", "20-ft. radius", {
          shape: "radius",
          dims: [qty(20, "foot")],
        }),
      ),
    ).toBe("20 ft radius");
    expect(
      areaText(
        fr,
        a("area", "10 x 60 ft.", { dims: [qty(10, "foot"), qty(60, "foot")] }),
      ),
    ).toBe("3 m × 18 m");
    expect(
      areaText(
        fr,
        a("targets", "1 creature/level", {
          targets: qty(1, undefined, { per_level: true, step: 1 }),
          noun: "creature",
        }),
      ),
    ).toBe("1 créature/niveau");
    expect(
      areaText(
        fr,
        a("area", "40-ft. cube, 1 creature/level", {
          shape: "cube",
          dims: [qty(40, "foot")],
          targets: qty(1, undefined, { per_level: true, step: 1 }),
          noun: "creature",
        }),
      ),
    ).toBe("cube de 12 m, 1 créature/niveau");
    expect(areaText(fr, a("special", "Special"))).toBe("Spéciale");
  });

  it("says a square of square units once, by the unit", () => {
    expect(
      areaText(
        fr,
        a("area", "10 sq. ft./level", {
          shape: "square",
          dims: [qty(10, "sq foot", { per_level: true, step: 1 })],
        }),
      ),
    ).toBe("0,9 m²/niveau");
    expect(
      areaText(
        fr,
        a("area", "10-ft. square", {
          shape: "square",
          dims: [qty(10, "foot")],
        }),
      ),
    ).toBe("carré de 3 m");
  });

  it("is null for a partial reading: the text says more than its typed form", () => {
    expect(
      areaText(
        fr,
        a("targets", "1 creature or item", {
          targets: qty(1),
          noun: "creature",
          partial: true,
        }),
      ),
    ).toBeNull();
  });

  it("is null where the typed value drops what the book says", () => {
    expect(areaText(fr, a("self", "The caster's hand"))).toBeNull();
    expect(areaText(fr, a("touch", "Creature touched"))).toBeNull();
    expect(
      areaText(
        fr,
        a("targets", "Creates 1d4+1", {
          targets: qty(1, undefined, { dice: "1d4" }),
        }),
      ),
    ).toBeNull();
    expect(
      areaText(
        fr,
        a("area", "10-ft. blob", { shape: "blob", dims: [qty(10, "foot")] }),
      ),
    ).toBeNull();
    expect(areaText(fr, a("special", "Line of sight"))).toBeNull();
  });
});

describe("componentsText", () => {
  const c = (
    raw: string,
    more: Partial<SpellStats["components"]>,
  ): SpellStats["components"] => ({
    raw,
    v: false,
    s: false,
    m: false,
    ...more,
  });

  it("words the letters and the optional material component", () => {
    expect(
      componentsText(fr, c("V, S, M", { v: true, s: true, m: true })),
    ).toBe("V, S, M");
    expect(componentsText(fr, c("S", { s: true }))).toBe("S");
    expect(
      componentsText(
        fr,
        c("V, S (M optional)", { v: true, s: true, m: true, m_optional: true }),
      ),
    ).toBe("V, S (M optionnel)");
    expect(
      componentsText(
        fr,
        c("V, S, special", { v: true, s: true, special: true }),
      ),
    ).toBeNull();
  });
});

describe("saveText", () => {
  it("words every kind and an alternative", () => {
    expect(saveText(fr, { raw: "None", kinds: ["none"] })).toBe("Aucune");
    expect(saveText(fr, { raw: "Neg.", kinds: ["negates"] })).toBe("Annule");
    expect(saveText(fr, { raw: "1/2", kinds: ["half"] })).toBe("½");
    expect(saveText(fr, { raw: "Special", kinds: ["special"] })).toBe(
      "Spéciale",
    );
    expect(
      saveText(fr, { raw: "Neg. or 1/2", kinds: ["negates", "half"] }),
    ).toBe("Annule ou ½");
  });

  it("is null for 'Special' with another word", () => {
    expect(
      saveText(fr, { raw: "None (special)", kinds: ["none", "special"] }),
    ).toBeNull();
    expect(saveText(fr, { raw: "Variable", kinds: ["special"] })).toBeNull();
  });
});

describe("statText", () => {
  const spell = {
    casting_time: "1",
    range: "60 yds. + 10 yds./level",
    duration: "Instantaneous",
    area_of_effect: "Creature touched",
    components: "V, S",
    saving_throw: "None",
    stats: {
      casting_time: { raw: "1", kind: "timed", time: qty(1, "segment") },
      range: {
        raw: "60 yds. + 10 yds./level",
        kind: "distance",
        distance: {
          terms: [
            { count: 60, unit: "yard" },
            { count: 10, unit: "yard", per_level: true, step: 1 },
          ],
        },
      },
      duration: { raw: "Instantaneous", kind: "instantaneous", time: none },
      area: { raw: "Creature touched", kind: "touch" },
      components: { raw: "V, S", v: true, s: true, m: false },
      save: { raw: "None", kinds: ["none"] },
    },
  } as Pick<Spell, "stats" | "range">;

  it("words the typed value in French", () => {
    expect(statText(fr, spell as Spell, "range")).toEqual({
      text: "60 m + 10 m/niveau",
      source: "typed",
    });
    expect(statText(fr, spell as Spell, "casting_time").text).toBe("1 segment");
  });

  it("falls back to the book's text, its common values labelled", () => {
    expect(statText(fr, spell as Spell, "area_of_effect")).toEqual({
      text: "Créature touchée",
      source: "label",
    });
  });

  it("is the book's text in English, as before", () => {
    expect(statText(en, spell as Spell, "range")).toEqual({
      text: "60 yds. + 10 yds./level",
      source: "book",
    });
    expect(statText(en, spell as Spell, "components").text).toBe("V, S");
  });
});
