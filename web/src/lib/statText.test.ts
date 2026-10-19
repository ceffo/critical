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
  "line of sight",
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
  "circle",
  "path",
  "line",
  "wave",
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
    expect(unitText(fr, "round", 1)).toBe("tour");
    expect(unitText(fr, "round", 3)).toBe("tours");
    expect(unitText(fr, "turn", 2)).toBe("tours");
    expect(unitText(fr, "hour", 1)).toBe("heure");
    expect(unitText(fr, "sq m", 4)).toBe("m²");
    expect(unitText(fr, "cu m", 4)).toBe("m³");
    expect(unitText(en, "round", 3)).toBe("rd");
    expect(unitText(en, "sq foot", 3)).toBe("sq ft");
  });

  it("is null for a word that is no unit", () => {
    expect(unitText(fr, "rum", 1)).toBeNull();
  });

  it('words a count of uses as a unit ("1 usage/2 levels")', () => {
    expect(unitText(fr, "usage", 1)).toBe("utilisation");
    expect(unitText(fr, "usage", 2)).toBe("utilisations");
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

  it("words the nouns the areas count with the French books' terms", () => {
    expect(nounText(fr, "oak club", 1)).toBe("gourdin en chêne");
    expect(nounText(fr, "sigil", 1)).toBe("sceau");
    expect(nounText(fr, "insects", 3)).toBe("insectes");
    expect(nounText(fr, "fresh berries", 2)).toBe("baies fraîches");
    expect(nounText(fr, "components", 3)).toBe("éléments");
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
    expect(quantityText(fr, qty(1, "foot", { per_level: true, step: 1 }))).toBe(
      "30 cm/niveau",
    );
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
    expect(formatValue(fr, { amount: 15, unit: "round" })).toBe("15 tours");
    expect(formatValue(fr, { amount: 1, unit: "turn" })).toBe("1 tour");
    expect(formatValue(fr, { amount: 8, dice: "1d8", unit: "round" })).toBe(
      "8 + 1d8 tours",
    );
    expect(formatValue(fr, { amount: 0, dice: "1d4", unit: "hour" })).toBe(
      "1d4 heures",
    );
    expect(formatValue(fr, { amount: 1.5, unit: "round" })).toBe("1,5 tour");
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
    ).toBe("1 tour/2 niveaux");
    expect(
      quantityText(fr, qty(2, "round", { per_level: true, step: 1 })),
    ).toBe("2 tours/niveau");
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
    ).toBe("1 tour");
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
    ).toBe("1 tour + 1 tour/niveau");
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
    ).toBe("Spéciale (jusqu'à 3 tours)");
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

  it("words the figure a radius or diameter measures", () => {
    const sphere = a("area", "10-ft. radius sphere", {
      shape: "radius",
      figure: "sphere",
      dims: [qty(10, "foot")],
    });
    expect(areaText(fr, sphere)).toBe("sphère de 3 m de rayon");
    expect(areaText(en, sphere)).toBe("10 ft radius sphere");
    expect(
      areaText(
        fr,
        a("alternatives", "", {
          alternatives: [
            sphere,
            a("area", "", {
              shape: "radius",
              figure: "hemisphere",
              dims: [qty(15, "foot")],
            }),
          ],
        }),
      ),
    ).toBe("sphère de 3 m de rayon ou hémisphère de 4,5 m de rayon");
    // More than the radius and one more extent with a figure: the book's text.
    expect(
      areaText(
        fr,
        a("area", "", {
          shape: "radius",
          figure: "column",
          dims: [qty(5, "foot"), qty(30, "foot"), qty(2, "foot")],
        }),
      ),
    ).toBeNull();
  });

  it("words a choice between readings, each in turn", () => {
    const alts = (...alternatives: SpellStats["area"][]) =>
      a("alternatives", "", { alternatives });
    const one = (noun: string) => a("targets", "", { targets: qty(1), noun });
    expect(areaText(fr, alts(one("creature"), one("object")))).toBe(
      "1 créature ou 1 objet",
    );
    expect(
      areaText(
        fr,
        alts(
          one("creature"),
          one("object"),
          a("area", "", { shape: "square", dims: [qty(10, "foot")] }),
        ),
      ),
    ).toBe("1 créature ou 1 objet ou carré de 3 m");
    expect(areaText(en, alts(one("creature"), one("object")))).toBe(
      "1 creature or 1 object",
    );
    // One reading the language cannot word: the book's text for the whole choice.
    expect(areaText(fr, alts(one("creature"), one("wyvern")))).toBeNull();
  });

  it('words "up to" on the figure, not on the length inside it', () => {
    const cube = a("area", "Up to 20-ft. cube", {
      shape: "cube",
      dims: [{ ...qty(20, "foot"), up_to: true }],
    });
    expect(areaText(fr, cube)).toBe("cube de 6 m maximum");
    expect(areaText(en, cube)).toBe("up to 20 ft cube");
    expect(
      areaText(
        fr,
        a("targets", "Up to 6 creatures", {
          targets: { ...qty(6), up_to: true },
          noun: "creatures",
        }),
      ),
    ).toBe("jusqu'à 6 créatures");
  });

  it("words the axis of each extent", () => {
    const path = a("area", "One 10'-wide path, 60 feet long", {
      shape: "path",
      dims: [
        { ...qty(10, "foot"), axis: "wide" },
        { ...qty(60, "foot"), axis: "long" },
      ],
    });
    expect(areaText(fr, path)).toBe(
      "chemin de 3 m de large et de 18 m de long",
    );
    const slab = a("area", "4 ft./level wide x 2 ft./level high", {
      dims: [
        { ...qty(4, "foot", { per_level: true, step: 1 }), axis: "wide" },
        { ...qty(2, "foot", { per_level: true, step: 1 }), axis: "high" },
      ],
    });
    expect(areaText(fr, slab)).toBe(
      "1,2 m/niveau de large et de 60 cm/niveau de haut",
    );
    const wall = a("area", "wall, 10 x 5 ft./level, 2 ft. wide", {
      shape: "wall",
      dims: [
        qty(10, "foot"),
        qty(5, "foot", { per_level: true, step: 1 }),
        { ...qty(2, "foot"), axis: "wide" },
      ],
    });
    expect(areaText(fr, wall)).toBe(
      "mur de 3 m × 1,5 m/niveau et de 60 cm de large",
    );
    expect(areaText(en, path)).toBe("10 ft wide and 60 ft long path");
  });

  it("words the size of a group after its noun", () => {
    const unit = a("targets", "One unit of up to 200 individuals", {
      targets: qty(1),
      noun: "unit",
      members: { ...qty(200), up_to: true },
      members_noun: "individuals",
    });
    expect(areaText(fr, unit)).toBe("1 unité de 200 individus maximum");
  });

  it("words targets within a distance, and an area around what is touched", () => {
    expect(
      areaText(
        fr,
        a("area", "1 animal within 30 ft.", {
          dims: [qty(30, "foot")],
          targets: qty(1),
          noun: "animal",
          within: true,
        }),
      ),
    ).toBe("1 animal dans un rayon de 9 m");
    expect(
      areaText(
        fr,
        a("area", "10-ft. radius around creature touched", {
          shape: "radius",
          dims: [qty(10, "foot")],
          around_touched: true,
        }),
      ),
    ).toBe("rayon de 3 m autour de la créature touchée");
    // Within a shape ("within a 20-foot cube"): the book's text.
    expect(
      areaText(
        fr,
        a("area", "", {
          shape: "cube",
          dims: [qty(20, "foot")],
          targets: qty(1),
          noun: "creature",
          within: true,
        }),
      ),
    ).toBeNull();
  });

  it("has a French noun for pebbles and eyes", () => {
    for (const [noun, text] of [
      ["pebbles", "3 galets"],
      ["eyes", "3 yeux"],
    ])
      expect(areaText(fr, a("targets", "", { targets: qty(3), noun }))).toBe(
        text,
      );
  });

  it("words several areas, a sum of readings and the notes the book adds", () => {
    const cubes = a("area", "Two 10-ft. cubes/level", {
      shape: "cube",
      count: 2,
      dims: [qty(10, "foot", { per_level: true, step: 1 })],
    });
    expect(areaText(fr, cubes)).toBe("2 cubes de 3 m/niveau");
    const sum = a("together", "One 40-ft. cube + one 10-ft. cube/level", {
      parts: [
        a("area", "One 40-ft. cube", {
          shape: "cube",
          dims: [qty(40, "foot")],
        }),
        a("area", "one 10-ft. cube/level", {
          shape: "cube",
          dims: [qty(10, "foot", { per_level: true, step: 1 })],
        }),
      ],
    });
    expect(areaText(fr, sum)).toBe("cube de 12 m + cube de 3 m/niveau");
    expect(
      areaText(
        fr,
        a("together", "Caster + 1 person/8 levels", {
          parts: [
            a("self", "Caster"),
            a("targets", "1 person/8 levels", {
              targets: qty(1, undefined, { per_level: true, step: 8 }),
              noun: "person",
            }),
          ],
        }),
      ),
    ).toBe("Le lanceur + 1 personne/8 niveaux");
    // A part the language cannot word leaves the book's text.
    expect(
      areaText(
        fr,
        a("together", "", {
          parts: [
            a("self", "Caster"),
            a("targets", "", { targets: qty(1), noun: "zzz" }),
          ],
        }),
      ),
    ).toBeNull();
    const radius = a("area", "5 ft. radius + special", {
      shape: "radius",
      dims: [qty(5, "foot")],
    });
    expect(areaText(fr, { ...radius, special: "plus" })).toBe(
      "rayon de 1,5 m + spéciale",
    );
    expect(areaText(fr, { ...radius, special: "or" })).toBe(
      "rayon de 1,5 m ou spéciale",
    );
    expect(areaText(fr, { ...radius, special: "note" })).toBe(
      "rayon de 1,5 m (spéciale)",
    );
    expect(
      areaText(
        fr,
        a("targets", "One individual (usually caster)", {
          targets: qty(1),
          noun: "individual",
          usually: "caster",
        }),
      ),
    ).toBe("1 individu (généralement le lanceur)");
  });

  it("words where on the figure an extent is measured, and a bounded line of sight", () => {
    const cone = a("area", "Cone 60 yards long and 30 yards wide at the base", {
      shape: "cone",
      dims: [
        { ...qty(60, "yard"), axis: "long" },
        { ...qty(30, "yard"), axis: "wide", site: "base" },
      ],
    });
    expect(areaText(fr, cone)).toBe(
      "cône de 60 m de long et de 30 m de large à la base",
    );
    // A site with no axis to say it of keeps the book's text.
    expect(
      areaText(
        fr,
        a("area", "", {
          shape: "cone",
          dims: [{ ...qty(5, "foot"), site: "base" }],
        }),
      ),
    ).toBeNull();
    expect(
      areaText(
        fr,
        a("area", "Line of sight, max. 60 ft.", {
          shape: "line of sight",
          dims: [{ ...qty(60, "foot"), up_to: true }],
        }),
      ),
    ).toBe("ligne de vue de 18 m maximum");
  });

  it("words the size of a target, a level times an area, a wave and a radius with a column", () => {
    // ADR 0021 step 8, areas 18-20
    const sized = a("targets", "1 object up to 10 ft. long", {
      size: true,
      targets: qty(1),
      noun: "object",
      dims: [{ ...qty(10, "foot"), axis: "long", up_to: true }],
    });
    expect(areaText(fr, sized)).toBe("1 objet, 3 m de long maximum");
    // Without the flag the extent is an area of its own, not the target's size.
    expect(areaText(fr, { ...sized, size: false })).toBe(
      "3 m de long maximum, 1 objet",
    );
    expect(
      areaText(
        fr,
        a("targets", "1 page, up to 2 ft. square", {
          size: true,
          shape: "square",
          targets: qty(1),
          noun: "page",
          dims: [{ ...qty(2, "foot"), up_to: true }],
        }),
      ),
    ).toBe("1 page, carré de 60 cm maximum");
    expect(
      areaText(
        fr,
        a("area", "caster's level x 100 sq. ft.", {
          shape: "square",
          dims: [qty(100, "sq foot", { per_level: true, step: 1 })],
        }),
      ),
    ).toBe("9 m²/niveau");
    expect(
      areaText(
        fr,
        a("area", "Wave 2 ft. high and 10 ft. long per level", {
          shape: "wave",
          dims: [
            { ...qty(2, "foot"), axis: "high" },
            { ...qty(10, "foot", { per_level: true, step: 1 }), axis: "long" },
          ],
        }),
      ),
    ).toBe("vague de 60 cm de haut et de 3 m/niveau de long");
    expect(
      areaText(
        fr,
        a("area", "5 ft. radius x 30 ft. column", {
          shape: "radius",
          figure: "column",
          dims: [qty(5, "foot"), qty(30, "foot")],
        }),
      ),
    ).toBe("colonne de 1,5 m de rayon sur 9 m");
    // Two extents with a figure but no radius are not read.
    expect(
      areaText(
        fr,
        a("area", "", {
          shape: "cone",
          figure: "column",
          dims: [qty(5, "foot"), qty(30, "foot")],
        }),
      ),
    ).toBeNull();
    const alts = a("alternatives", "1 speaking creature or written text", {
      alternatives: [
        a("targets", "1 speaking creature", {
          targets: qty(1),
          noun: "speaking creature",
        }),
        a("targets", "written text", { targets: qty(1), noun: "written text" }),
      ],
    });
    expect(areaText(fr, alts)).toBe("1 créature parlante ou 1 objet écrit");
  });

  it('words "1 or more" after the noun', () => {
    expect(
      areaText(
        fr,
        a("area", "1 or more creatures in a 10-ft. radius", {
          shape: "radius",
          dims: [qty(10, "foot")],
          targets: { ...qty(1), or_more: true },
          noun: "creatures",
        }),
      ),
    ).toBe("rayon de 3 m, 1 créature ou plus");
  });

  it("words a non-level divisor the language lists, and no other", () => {
    const sq = (per: string) =>
      a("area", "", {
        shape: "square",
        dims: [{ terms: [{ count: 10, unit: "yard", per }] }],
      });
    expect(areaText(fr, sq("priest"))).toBe("carré de 10 m/prêtre");
    expect(areaText(fr, sq("wyvern"))).toBeNull();
    expect(
      areaText(
        fr,
        a("targets", "1 creature/rd.", {
          targets: { terms: [{ count: 1, per: "round" }] },
          noun: "creature",
        }),
      ),
    ).toBe("1 créature/tour");
  });

  it("words dice and a cap in an extent", () => {
    expect(
      areaText(
        fr,
        a("area", "4d4 sq. mi.", {
          dims: [{ terms: [{ count: 0, dice: "4d4", unit: "sq mile" }] }],
        }),
      ),
    ).toBe("4d4 milles carrés");
    expect(
      areaText(
        fr,
        a("area", "5 yds./level (90 yds. maximum)", {
          dims: [
            {
              terms: [{ count: 5, unit: "yard", per_level: true, step: 1 }],
              max: 90,
              max_unit: "yard",
            },
          ],
        }),
      ),
    ).toBe("5 m/niveau, max. 90 m");
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
    casting_time: { en: "1" },
    range: { en: "60 yds. + 10 yds./level" },
    duration: { en: "Instantaneous" },
    area_of_effect: { en: "Creature touched" },
    components: { en: "V, S" },
    saving_throw: { en: "None" },
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
