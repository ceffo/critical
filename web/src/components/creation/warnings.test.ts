import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { CharacterWarning } from "@/types/character";
import { warnedFields } from "./warnings";

describe("warnedFields", () => {
  it("leaves out the slot warnings of every pool, weapon ones included (ADR 0037 step 11)", async () => {
    const inst = i18next.createInstance();
    await inst.init({
      lng: "fr",
      defaultNS: "common",
      resources,
      interpolation: { escapeValue: false },
    });
    const warnings: CharacterWarning[] = [
      {
        code: "slots_to_spend",
        field: "nonweapon",
        pool: "nonweapon",
        left: 2,
      },
      { code: "slots_to_spend", field: "weapon", pool: "weapon", left: 1 },
      {
        code: "slots_over_budget",
        field: "weapon",
        pool: "weapon",
        spent: 6,
        available: 4,
      },
      {
        code: "height_outside_range",
        field: "height",
        value: 250,
        min: 150,
        max: 190,
      },
    ];
    expect(warnedFields(inst.t, warnings, 0)).toEqual(["taille"]);
  });
});
