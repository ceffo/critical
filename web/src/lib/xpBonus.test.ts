import { describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { bonusHint } from "./xpBonus";

// ADR 0042 step 6: the sentence names the prime requisites the server sends for the active
// classes, and only those.
const NAMES = {
  en: {
    str: "Strength",
    dex: "Dexterity",
    con: "Constitution",
    int: "Intelligence",
    wis: "Wisdom",
    cha: "Charisma",
  },
  fr: {
    str: "la Force",
    dex: "la Dextérité",
    con: "la Constitution",
    int: "l’Intelligence",
    wis: "la Sagesse",
    cha: "le Charisme",
  },
} as const;
type Code = keyof (typeof NAMES)["en"];
const CODES = Object.keys(NAMES.en) as Code[];

const CLASSES: Record<string, Code[]> = {
  fighter: ["str"],
  cleric: ["wis"],
  mage: ["int"],
  thief: ["dex"],
  psionicist: ["int", "wis", "con"],
  bard: ["dex", "cha"],
};

for (const lang of ["en", "fr"] as const) {
  const t = i18n.getFixedT(lang);
  const words = (primes: string[]) =>
    bonusHint(t, lang, {
      bonusClasses: [{ class: "x", primes }],
      bonusMinScore: 16,
    });

  describe(`bonusHint (${lang})`, () => {
    for (const [cls, primes] of Object.entries(CLASSES)) {
      it(`${cls}: names its own prime requisites and no other ability`, () => {
        const hint = words(primes) ?? "";
        for (const a of CODES)
          if (primes.includes(a)) expect(hint).toContain(NAMES[lang][a]);
          else expect(hint).not.toContain(NAMES[lang][a]);
        expect(hint).toContain("16");
      });
    }

    it("a multi-class character: the union of its classes' primes, once each", () => {
      const hint = bonusHint(t, lang, {
        bonusClasses: [
          { class: "fighter", primes: ["str"] },
          { class: "mage", primes: ["int"] },
          { class: "bard", primes: ["str", "dex"] },
        ],
        bonusMinScore: 16,
      });
      expect(hint).not.toBeNull();
      for (const a of ["str", "int", "dex"] as const)
        expect(hint!.split(NAMES[lang][a])).toHaveLength(2);
      expect(hint).not.toContain(NAMES[lang].cha);
    });

    it("no class with a bonus rule: no sentence", () => {
      expect(
        bonusHint(t, lang, { bonusClasses: [], bonusMinScore: 16 }),
      ).toBeNull();
    });
  });
}

describe("bonusHint wording", () => {
  it("keeps the bard's sentence", () => {
    expect(
      bonusHint(i18n.getFixedT("en"), "en", {
        bonusClasses: [{ class: "bard", primes: ["dex", "cha"] }],
        bonusMinScore: 16,
      }),
    ).toBe("+10% on XP earned when Dexterity and Charisma are all 16 or more");
  });
});
