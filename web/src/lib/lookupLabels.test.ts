import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  classLabel,
  labelKey,
  schoolLabel,
  sphereLabel,
  spheresLabel,
  componentsText,
  statCell,
  statText,
  translateValue,
} from "./lookupLabels";
import type { Spell } from "@/types/spells";

async function tFor(lng: "en" | "fr") {
  const i = i18next.createInstance();
  await i.init({
    lng,
    resources,
    defaultNS: "common",
    interpolation: { escapeValue: false },
  });
  return i.t;
}

describe("lookupLabels", () => {
  it("builds a key from the trimmed, lowercased text", () => {
    expect(labelKey("  The Caster ")).toBe("the_caster");
    expect(labelKey("Neg.")).toBe("neg_");
  });

  it("translates a listed value, ignoring case and surrounding spaces", async () => {
    const t = await tFor("fr");
    expect(translateValue(t, "duration", "Instantaneous")).toBe("Instantanée");
    expect(translateValue(t, "duration", "  instantaneous ")).toBe(
      "Instantanée",
    );
    expect(translateValue(t, "area", "The caster")).toBe("Le lanceur");
    expect(translateValue(t, "range", "Touch")).toBe("Contact");
    expect(translateValue(t, "save", "None")).toBe("Aucune");
  });

  it("leaves unlisted and typed values exactly as scraped", async () => {
    const t = await tFor("fr");
    expect(translateValue(t, "range", "210 yd")).toBe("210 yd");
    expect(translateValue(t, "range", "10 yds./level")).toBe("10 yds./level");
    expect(translateValue(t, "duration", "Whenever Needed")).toBe(
      "Whenever Needed",
    );
    expect(translateValue(t, "area", "")).toBe("");
    // a value listed for another stat only is not borrowed
    expect(translateValue(t, "range", "Permanent")).toBe("Permanent");
  });

  it("translates classes and spheres, one sphere name at a time", async () => {
    const t = await tFor("fr");
    expect(classLabel(t, "Wizard")).toBe("Magicien");
    expect(classLabel(t, "Priest")).toBe("Prêtre");
    expect(sphereLabel(t, "All")).toBe("Tout");
    expect(sphereLabel(t, "Elemental Fire")).toBe("Élémentaire (Feu)");
    expect(sphereLabel(t, "Unknown Sphere")).toBe("Unknown Sphere");
    expect(spheresLabel(t, "Numbers, Divination")).toBe("Nombres, Divination");
  });

  it("shows the English text in English", async () => {
    const t = await tFor("en");
    expect(translateValue(t, "duration", "Instantaneous")).toBe(
      "Instantaneous",
    );
    expect(classLabel(t, "Priest")).toBe("Priest");
    expect(sphereLabel(t, "Elemental Fire")).toBe("Elemental Fire");
  });
});

describe("lookupLabels in French, as the lookup shows them", () => {
  it("translates the Priest values and the sphere a spell lists", async () => {
    const t = await tFor("fr");
    expect(translateValue(t, "castingTime", "Special")).toBe("Spécial");
    expect(translateValue(t, "range", "Touch")).toBe("Contact");
    expect(spheresLabel(t, "Sun")).toBe("Soleil");
    expect(classLabel(t, "Wizard")).toBe("Magicien");
  });

  it("leaves text with no entry as scraped", async () => {
    const t = await tFor("fr");
    expect(translateValue(t, "range", "Sight line")).toBe("Sight line");
    expect(translateValue(t, "area", "A weird zone")).toBe("A weird zone");
  });
});

// Every distinct school text of the spell catalogue (scraped, typos included).
const CATALOGUE_SCHOOLS = [
  "Abjuration",
  "Abjuration, Alteration",
  "Abjuration, Conjuration",
  "Abjuration, Conjuration/Summoning",
  "Abjuration, Elemental Air, Alchemy",
  "Abjuration, Elemental Fire",
  "Abjuration, Enchantment",
  "Abjuration, Evocation",
  "Abjuration, Geometry",
  "Abjuration, Invocation/Evocation",
  "Abjuration, Mentalism",
  "Abjuration, Necromancy",
  "All Schools",
  "Alteration",
  "Alteration, Abjuration",
  "Alteration, Conjuration",
  "Alteration, Conjuration/Summoning",
  "Alteration, Dimension",
  "Alteration, Elemental Earth",
  "Alteration, Elemental Water, Alchemy",
  "Alteration, Enchantment",
  "Alteration Enchantment/Charm",
  "Alteration, Enchantment/Charm",
  "Alteration, Enchantment, Evocation",
  "Alteration, Evocation",
  "Alteration, Geometry, Artifice",
  "Alteration, Illusion",
  "Alteration, Illusion/Phantasm",
  "Alteration, Invocation",
  "Alteration, Invocation/Evocation",
  "Alteration, Necromancy",
  "Alteration, Phantasm",
  "Conjuration",
  "Conjuration, Alteration",
  "Conjuration, Necromancy",
  "Conjuration, Phantasm",
  "(Conjuration/Summoning",
  "Conjuration, Summoning",
  "Conjuration/Summoning",
  "Conjuration/Summoning, Alchemy",
  "Conjuration/Summoning, all Elemental, Geometry",
  "Conjuration/Summoning, Dimension",
  "Conjuration/Summoning, Elemental Water, Alchemy",
  "Conjuration/Summoning, Enchantment",
  "Conjuration/Summoning, Invocation/Evocation",
  "Conjuration/Summoning, Necromancy",
  "Conjuration/Summoning, Shadow",
  "Dimension",
  "Divination",
  "Divination, Alteration",
  "Divination, Artifice",
  "Divination, Dimension",
  "Divination, Enchantment",
  "Divination, Enchantment/Charm",
  "Divination/Illusion",
  "Divination, Necromancy",
  "Divination, Summoning",
  "Enchantment",
  "Enchantment, Alteration",
  "Enchantment/Charm",
  "Enchantment/Charm, Alteration",
  "Enchantment/Charm, Illusion/Phantasm",
  "Enchantment/Charm, Mentalism",
  "Enchantment/Charm, Song",
  "Enchantment, Divination",
  "Enchantment, Evocation",
  "Enchantment, Invocation",
  "Enchantment, Necromancy",
  "Evocation",
  "Evocation, Abjuration",
  "Evocation, Alteration",
  "Evocation, Alteration, Enchantment/Charm",
  "Evocation, Enchantment",
  "Evocation, Enchantment/Charm",
  "Evocation, Illusion/Phantasm",
  "Evocation, Necromancy",
  "Illusion",
  "Illusion, Enchantment",
  "Illusion,  Necromancy",
  "Illusion/Phantasm",
  "Illusion/Phantasm, Abjuration",
  "Illusion/Phantasm, Alteration",
  "Invocation",
  "Invocation/Evocation",
  "Invocation/Evocation, Dimension, Artifice",
  "Invocation/Evocation, Elemental Air, Force",
  "Invocation/Evocation, Elemental Water",
  "Invocation/Evocation, Force",
  "Invocation/Evocation, Geometry",
  "Invocation, Illusion/Phantasm",
  "Necromancy",
  "Necromancy, Alteration",
  "Necromancy/Alteration",
  "Necromancy, Artifice",
  "Necromancy, Conjuration",
  "Necromancy, Conjuration/Summoning",
  "Necromancy, Divination",
  "Necromancy, Enchantment",
  "Necromancy, Evocation",
  "Necromantic",
  "Summoning, Invocation",
  "Various schools",
];

describe("schoolLabel", () => {
  it("translates every catalogue school: no English word is left in French", async () => {
    const t = await tFor("fr");
    const english =
      /\b(Alteration|Summoning|Charm|Phantasm|Necromancy|Elemental|Mentalism|Shadow|Song|Alchemy|Geometry|Schools|Various)\b/;
    for (const raw of CATALOGUE_SCHOOLS) {
      const out = schoolLabel(t, raw);
      expect(out, raw).not.toMatch(english);
      expect(out, raw).not.toMatch(/[(]/);
    }
  });

  it("shows English unchanged apart from cleanup", async () => {
    const t = await tFor("en");
    for (const raw of CATALOGUE_SCHOOLS) {
      expect(schoolLabel(t, raw), raw).toBe(
        raw
          .replace(/^\(/, "")
          .replace(/,\s+/g, ", ")
          .replace(" Enchantment/Charm", " Enchantment/Charm"),
      );
    }
  });

  it("splits several schools and translates each", async () => {
    const t = await tFor("fr");
    expect(schoolLabel(t, "Alteration, Elemental Water, Alchemy")).toBe(
      "Altération, Eau élémentaire, Alchimie",
    );
    expect(schoolLabel(t, "Invocation/Evocation, Force")).toBe(
      "Invocation/Évocation, Force",
    );
    expect(schoolLabel(t, "Necromancy/Alteration")).toBe(
      "Nécromancie/Altération",
    );
    expect(schoolLabel(t, "Alteration Enchantment/Charm")).toBe(
      "Altération Enchantement/Charme",
    );
  });

  it("leaves an unknown school as scraped", async () => {
    const t = await tFor("fr");
    expect(schoolLabel(t, "Chronomancy")).toBe("Chronomancy");
    expect(schoolLabel(t, "")).toBe("");
  });
});

describe("statText", () => {
  // Magic Missile with French text for its range and duration only (the fixture's shape): the
  // other stats fall back to English, per field.
  const mm = {
    casting_time: "1",
    range: "60 yds. + 10 yds./level",
    duration: "Instantaneous",
    area_of_effect: "1-5 targets",
    components: "V, S",
    saving_throw: "None",
    range_fr: "60 mètres + 10 mètres par niveau",
    duration_fr: "Instantanée immédiate",
    components_fr: "V, S (M optionnel)",
  } as Spell;

  it("shows the catalogue's French text in French", async () => {
    const t = await tFor("fr");
    expect(statText(t, "fr", mm, "range")).toBe(
      "60 mètres + 10 mètres par niveau",
    );
    // the French text wins over the label of the English value
    expect(statText(t, "fr", mm, "duration")).toBe("Instantanée immédiate");
    expect(componentsText("fr", mm)).toBe("V, S (M optionnel)");
  });

  it("falls back to the English text per field, its common values labelled", async () => {
    const t = await tFor("fr");
    expect(statText(t, "fr", mm, "castingTime")).toBe("1");
    expect(statText(t, "fr", mm, "area")).toBe("1-5 targets");
    expect(statText(t, "fr", mm, "save")).toBe("Aucune");
    expect(
      componentsText("fr", { ...mm, components_fr: undefined } as Spell),
    ).toBe("V, S");
  });

  it("is English in English, even with French texts", async () => {
    const t = await tFor("en");
    expect(statText(t, "en", mm, "range")).toBe("60 yds. + 10 yds./level");
    expect(statText(t, "en", mm, "duration")).toBe("Instantaneous");
    expect(componentsText("en", mm)).toBe("V, S");
  });
});

describe("statCell", () => {
  const spell = {
    casting_time: "1",
    range: "60 yds. + 10 yds./level",
    duration: "Instantaneous",
    area_of_effect: "1 creature",
    components: "V, S",
    saving_throw: "None",
    range_fr: "60 pas + 10 pas/niveau",
    area_of_effect_fr: "1 créature",
    saving_throw_fr: "Aucun",
  } as Spell;

  it("drops a value that only repeats the English text when the French text is there", async () => {
    const t = await tFor("fr");
    expect(statCell(t, "fr", spell, "area", "1 creature")).toEqual({
      value: null,
      text: "1 créature",
    });
    // the save's value is its kinds, labelled: "Aucune" is the French label of "None"
    expect(statCell(t, "fr", spell, "save", "Aucune")).toEqual({
      value: null,
      text: "Aucun",
    });
  });

  it("keeps a value that depends on the level, the French text under it", async () => {
    const t = await tFor("fr");
    expect(statCell(t, "fr", spell, "range", "210 yd")).toEqual({
      value: "210 yd",
      text: "60 pas + 10 pas/niveau",
    });
  });

  it("without a French text: the value, and the English text unless it says the same", async () => {
    const t = await tFor("fr");
    expect(statCell(t, "fr", spell, "castingTime", "1 seg")).toEqual({
      value: "1 seg",
      text: "1",
    });
    expect(statCell(t, "fr", spell, "duration", null)).toEqual({
      value: null,
      text: "Instantanée",
    });
    const en = await tFor("en");
    expect(statCell(en, "en", spell, "area", "1 creature")).toEqual({
      value: "1 creature",
      text: "",
    });
    expect(statCell(en, "en", spell, "range", "210 yd")).toEqual({
      value: "210 yd",
      text: "60 yds. + 10 yds./level",
    });
  });
});
