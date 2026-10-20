import { describe, expect, it } from "vitest";
import i18n from "@/i18n";
import {
  buffName,
  englishSpellName,
  spellName,
  spellId,
  spellNamer,
  spellTexts,
} from "./spellName";
import type { SpellNames } from "@/types/spells";

const names: SpellNames = {
  magic_missile: {
    id: "magic_missile",
    name: { en: "Magic Missile", fr: "Projectile magique" },
  },
  armor: { id: "armor", name: { en: "Armor" } },
};

describe("spellNamer", () => {
  it("gives the catalogue's French name in French", () => {
    expect(spellNamer("fr", names)("magic_missile", "Magic Missile")).toBe(
      "Projectile magique",
    );
  });

  it("falls back to English for a spell with no French name", () => {
    expect(spellNamer("fr", names)("armor", "Armor")).toBe("Armor");
  });

  it("gives the catalogue's English name in English, before the supplied one", () => {
    expect(spellNamer("en", names)("magic_missile", "Magic missile")).toBe(
      "Magic Missile",
    );
  });

  it("falls back to the supplied English name, then the id, for an id the catalogue lacks", () => {
    const fr = spellNamer("fr", names);
    expect(fr("unknown_spell", "Unknown Spell")).toBe("Unknown Spell");
    expect(fr("unknown_spell")).toBe("unknown_spell");
    expect(spellNamer("fr", {})("magic_missile", "Magic Missile")).toBe(
      "Magic Missile",
    );
  });

  it("englishSpellName names from what the caller holds", () => {
    expect(englishSpellName("magic_missile", "Magic Missile")).toBe(
      "Magic Missile",
    );
    expect(englishSpellName("magic_missile")).toBe("magic_missile");
  });
});

describe("spellTexts", () => {
  const spell = {
    name: { en: "Magic Missile", fr: "Projectile magique" },
    description: { en: "English text." },
  };
  const both = {
    ...spell,
    description: { en: "English text.", fr: "Texte français." },
  };

  it("picks French per field and falls back to English for the missing one", () => {
    expect(spellTexts("fr", spell)).toEqual({
      name: "Projectile magique",
      description: "English text.",
    });
    expect(spellTexts("fr", both)).toEqual({
      name: "Projectile magique",
      description: "Texte français.",
    });
  });

  it("is English in English", () => {
    expect(spellTexts("en", both)).toEqual({
      name: "Magic Missile",
      description: "English text.",
    });
  });
});

describe("spellName", () => {
  const mm = { name: { en: "Magic Missile", fr: "Projectile magique" } };

  it("gives the name in the UI language only", () => {
    expect(spellName("en", mm)).toBe("Magic Missile");
    expect(spellName("fr", mm)).toBe("Projectile magique");
    expect(spellName("fr-CA", mm)).toBe("Projectile magique");
  });

  it("falls back to English when the catalogue has no French name", () => {
    expect(spellName("fr", { name: { en: "Armor" } })).toBe("Armor");
    expect(spellName("fr", { name: { en: "Armor", fr: "" } })).toBe("Armor");
    expect(spellName("en", { name: { en: "Armor" } })).toBe("Armor");
  });
});

describe("spellId", () => {
  it("slugifies English names", () => {
    expect(spellId("Magic Missile")).toBe("magic_missile");
    expect(spellId("Tenser's Floating Disc")).toBe("tensers_floating_disc");
  });
});

describe("buffName", () => {
  const fr = spellNamer("fr", names);
  it("names a buff a proficiency check put on from the proficiency (ADR 0037 step 20)", () => {
    const french = i18n.getFixedT("fr");
    expect(
      buffName(fr, { name: "tumbling", proficiencyId: "tumbling" }, french),
    ).toBe("Acrobatie");
    expect(
      buffName(fr, { name: "gone", proficiencyId: "gone" }, french),
      "an id with no name keeps the stored one",
    ).toBe("gone");
  });

  it("names a buff from a spell from the catalogue", () => {
    expect(
      buffName(fr, { name: "Magic Missile", spellId: "magic_missile" }),
    ).toBe("Projectile magique");
  });

  it("keeps the stored name of a buff not from a spell", () => {
    expect(buffName(fr, { name: "Magic Missile" })).toBe("Magic Missile");
    expect(buffName(fr, { name: "Power Gem", spellId: "" })).toBe("Power Gem");
  });

  it("falls back to the stored name for a spell the catalogue does not know", () => {
    expect(buffName(fr, { name: "Old Spell", spellId: "gone" })).toBe(
      "Old Spell",
    );
  });
});
