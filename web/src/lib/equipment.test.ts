import { describe, expect, it } from "vitest";
import type { TFunction } from "i18next";
import type { Item } from "@/types/items";
import { spellNamer } from "@/i18n/spellName";
import { pieceStats } from "./equipment";

// Keys come back as themselves, so the test reads the order of the parts, not the wording.
const t = ((k: string) => k) as unknown as TFunction;

const names = {
  burning_hands: {
    id: "burning_hands",
    name: { en: "Burning Hands", fr: "Mains brûlantes" },
  },
};

function scroll(spellId: string): Item {
  return {
    category: "magic",
    magicKind: "scroll",
    behaviour: "stock",
    spellId,
    damageTypes: [],
  } as unknown as Item;
}

const texts = (i: Item, lang: string) =>
  pieceStats(i, t, spellNamer(lang, names)).map((p) => p.text);

describe("pieceStats: the spell a scroll holds (ADR 0033 step 2)", () => {
  it("names the spell in the player's language, after the kind", () => {
    expect(texts(scroll("burning_hands"), "en")).toEqual([
      "equipment.magicKinds.scroll",
      "Burning Hands",
      "items.behaviour.stock",
    ]);
    expect(texts(scroll("burning_hands"), "fr")[1]).toBe("Mains brûlantes");
  });

  it("adds nothing for an unnamed scroll", () => {
    expect(texts(scroll(""), "fr")).toEqual([
      "equipment.magicKinds.scroll",
      "items.behaviour.stock",
    ]);
  });

  it("falls back to the id when the catalogue lacks the spell", () => {
    expect(texts(scroll("lost_spell"), "en")[1]).toBe("lost_spell");
  });

  it("is only for a scroll", () => {
    const wand = { ...scroll("burning_hands"), magicKind: "wand" } as Item;
    expect(texts(wand, "en")).not.toContain("Burning Hands");
  });
});
