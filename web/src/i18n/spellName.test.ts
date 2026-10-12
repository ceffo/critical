import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { spellId, spellName } from "./spellName";

async function makeT(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    ns: ["common", "spells"],
    defaultNS: "common",
    resources: {
      en: { spells: { magic_missile: "Magic Missile", shield: "Shield" } },
      fr: { spells: { magic_missile: "Projectile magique" } },
    },
  });
  return inst.t;
}

describe("spellName", () => {
  it("uses the active language when the key exists", async () => {
    expect(spellName(await makeT("fr"), "magic_missile")).toBe(
      "Projectile magique",
    );
  });

  it("falls back to English when the fr key is missing", async () => {
    expect(spellName(await makeT("fr"), "shield")).toBe("Shield");
  });

  it("falls back to the supplied English name, then the id", async () => {
    const t = await makeT("fr");
    expect(spellName(t, "unknown_spell", "Unknown Spell")).toBe(
      "Unknown Spell",
    );
    expect(spellName(t, "unknown_spell")).toBe("unknown_spell");
  });
});

describe("spellId", () => {
  it("slugifies English names", () => {
    expect(spellId("Magic Missile")).toBe("magic_missile");
    expect(spellId("Tenser's Floating Disc")).toBe("tensers_floating_disc");
  });
});
