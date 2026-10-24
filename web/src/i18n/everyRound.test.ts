import i18next from "i18next";
import { describe, expect, it } from "vitest";
import { resources } from "@/i18n";

// An upkeep or an attack rate due every round reads "every round", never "every 1 rounds".

async function translator(lng: "en" | "fr") {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t.bind(inst);
}

describe("every-round wording", () => {
  it("words the kept-up power on the card and in the journal", async () => {
    const en = await translator("en");
    const fr = await translator("fr");
    for (const key of ["powers.use.keptUp", "log.powerParts.maintained"]) {
      expect(en(key, { cost: 2, count: 1 })).toBe(
        "kept up: 2 PSPs every round",
      );
      expect(en(key, { cost: 2, count: 3 })).toBe(
        "kept up: 2 PSPs every 3 rounds",
      );
      expect(fr(key, { cost: 2, count: 1 })).toBe(
        "maintenu : 2 PFP à chaque tour",
      );
      expect(fr(key, { cost: 2, count: 3 })).toBe(
        "maintenu : 2 PFP tous les 3 tours",
      );
    }
  });

  it("words the attack rate hint when the rate is per round", async () => {
    const en = await translator("en");
    const fr = await translator("fr");
    const rate = { attacks: 2, count: 1 };
    expect(en("attack.rateHintEveryRound", rate)).toBe(
      "Attacks per round: 2 every round; this round: 1",
    );
    expect(fr("attack.rateHintEveryRound", rate)).toBe(
      "Attaques par tour : 2 à chaque tour ; ce tour : 1",
    );
    expect(en("attack.rateHint", { ...rate, rounds: 2 })).toBe(
      "Attacks per round: 2 every 2 rounds; this round: 1",
    );
  });
});
