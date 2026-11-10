import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { powerEffectView } from "./powerEffect";

async function makeT(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t;
}

describe("powerEffectView (ADR 0043 step 17)", () => {
  it("words a signed AC with the buff form's words (en)", async () => {
    const t = await makeT("en");
    expect(powerEffectView(t, { ac: -2 })).toEqual({
      lines: [{ key: "ac", text: "AC −2" }],
      notes: [],
    });
  });

  it("words the weapon kind beside the numbers (fr)", async () => {
    const t = await makeT("fr");
    expect(
      powerEffectView(t, {
        to_hit: 1,
        damage: 1,
        weapon_kind: "melee",
      }),
    ).toEqual({
      lines: [
        { key: "to_hit", text: "Toucher +1" },
        { key: "damage", text: "Dégâts +1" },
      ],
      notes: ["attaques au corps à corps seulement"],
    });
  });

  it("says nothing for a power without an effect", async () => {
    const t = await makeT("en");
    expect(powerEffectView(t, undefined)).toEqual({ lines: [], notes: [] });
  });
});
