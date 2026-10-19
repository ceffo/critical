import { beforeAll, describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  mark,
  markedField,
  markedString,
  plainText,
  valueSpans,
  type ValueSpan,
} from "./valueMarks";
import {
  durationText,
  formatValue,
  quantityText,
  statLocale,
  statText,
  type StatLocale,
} from "./statText";
import type { Spell, SpellQuantity } from "@/types/spells";

const locales: Record<"en" | "fr", { plain: StatLocale; marked: StatLocale }> =
  {} as never;
beforeAll(async () => {
  for (const lng of ["en", "fr"] as const) {
    const i = i18next.createInstance();
    await i.init({
      lng,
      resources,
      defaultNS: "common",
      interpolation: { escapeValue: false },
    });
    locales[lng] = {
      plain: statLocale(i.t, lng),
      marked: statLocale(i.t, lng, true),
    };
  }
});

/** The values of a marked text as "kind:text". */
const values = (s: string) =>
  valueSpans(s)
    .filter((x) => x.kind)
    .map((x) => `${x.kind}:${x.text}`);

describe("value marks", () => {
  it("round-trips spans, and plainText drops the marks", () => {
    const spans: ValueSpan[] = [
      { text: "Deals " },
      { text: "1d6", kind: "dice" },
      { text: " for " },
      { text: "2 rounds/level", kind: "time" },
      { text: "." },
    ];
    const s = markedString(spans);
    expect(valueSpans(s)).toEqual(spans);
    expect(plainText(s)).toBe("Deals 1d6 for 2 rounds/level.");
  });

  it("serves the spans of the language the text was picked in, else the plain text", () => {
    const text = { en: "Lasts 2 rounds.", fr: "Dure 2 tours." };
    const marked = {
      description: {
        fr: [
          { text: "Dure " },
          { text: "2 tours", kind: "time" as const },
          { text: "." },
        ],
      },
    };
    expect(values(markedField("fr", text, marked, "description"))).toEqual([
      "time:2 tours",
    ]);
    // English has no marks here: the plain text, not the French spans.
    expect(markedField("en", text, marked, "description")).toBe(
      "Lasts 2 rounds.",
    );
    // A French field missing falls back to English, and takes the English spans.
    expect(
      values(
        markedField(
          "fr",
          { en: "Lasts 2 rounds." },
          {
            description: {
              en: [
                { text: "Lasts " },
                { text: "2 rounds", kind: "time" },
                { text: "." },
              ],
            },
          },
          "description",
        ),
      ),
    ).toEqual(["time:2 rounds"]);
  });

  it("ignores spans that do not make the text (a stale payload)", () => {
    const marked = {
      description: { en: [{ text: "Other.", kind: "dice" as const }] },
    };
    expect(markedField("en", { en: "Text." }, marked, "description")).toBe(
      "Text.",
    );
  });

  it("never lets a served text forge a mark", () => {
    expect(plainText(mark("dice", "ab"))).toBe("ab");
  });
});

describe("stats worded with their values marked (composition, not recognition)", () => {
  const per: SpellQuantity = {
    terms: [
      { count: 10, unit: "turn" },
      { count: 10, unit: "turn", per_level: true, step: 1 },
    ],
  };

  it("marks a term with a unit whole, per level included, in both languages", () => {
    expect(values(quantityText(locales.fr.marked, per)!)).toEqual([
      "time:100 tours",
      "time:100 tours/niveau",
    ]);
    expect(quantityText(locales.fr.plain, per)).toBe(
      plainText(quantityText(locales.fr.marked, per)!),
    );
  });

  it("marks a length, converted to metres in French", () => {
    const q: SpellQuantity = { terms: [{ count: 30, unit: "foot" }] };
    expect(values(quantityText(locales.fr.marked, q)!)).toEqual([
      "distance:9 m",
    ]);
    expect(values(quantityText(locales.en.marked, q)!)).toEqual([
      "distance:30 ft",
    ]);
  });

  it("marks only the dice and the per-level of a count of creatures", () => {
    const q: SpellQuantity = {
      terms: [{ count: 0, dice: "1d4", per_level: true, step: 1 }],
    };
    const text = quantityText(locales.fr.marked, q, "creature")!;
    expect(values(text)).toEqual(["dice:1d4", "level:/niveau"]);
    expect(plainText(text)).toBe(quantityText(locales.fr.plain, q, "creature"));
  });

  it("marks a value at the caster level", () => {
    const v = { amount: 8, unit: "turn", dice: "1d8" };
    expect(values(formatValue(locales.fr.marked, v))).toHaveLength(1);
    expect(plainText(formatValue(locales.fr.marked, v))).toBe(
      formatValue(locales.fr.plain, v),
    );
  });

  it("marks the typed duration, and the book's text with the API's spans", () => {
    const stats = {
      duration: { kind: "timed", time: per, raw: "10 turns + 10 turns/level" },
    } as unknown as Spell["stats"];
    expect(
      values(durationText(locales.fr.marked, stats.duration)!),
    ).toHaveLength(2);
    const spell = {
      casting_time: { en: "1 round" },
      range: {},
      duration: {},
      area_of_effect: {},
      components: {},
      saving_throw: {},
      stats,
      marked: {
        casting_time: { en: [{ text: "1 round", kind: "time" as const }] },
      },
    } as unknown as Spell;
    const t = statText(locales.en.marked, spell, "casting_time");
    expect(t.source).toBe("book");
    expect(values(t.text)).toEqual(["time:1 round"]);
    expect(statText(locales.en.plain, spell, "casting_time").text).toBe(
      "1 round",
    );
  });
});
