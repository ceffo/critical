import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  classLabel,
  labelKey,
  sphereLabel,
  spheresLabel,
  translateValue,
} from "./lookupLabels";

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
