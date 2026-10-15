import { describe, expect, it } from "vitest";
import { statDice, valueFormula } from "./spellValues";
import type { Spell, SpellStats, SpellValues } from "@/types/spells";

const unit = (w: string) => ({ round: "rd", yard: "yd" })[w] ?? w;
const q = { terms: [] };

function spell(values: SpellValues | undefined, area: Partial<SpellStats["area"]> = {}): Spell {
  const stats: SpellStats = {
    casting_time: { raw: "1", kind: "timed", time: q },
    range: { raw: "60 yds.", kind: "distance", distance: q },
    duration: { raw: "8 rds. + 1d8 rds.", kind: "timed", time: q },
    area: { raw: "1d4 persons", kind: "targets", noun: "persons", ...area },
    components: { raw: "V", v: true, s: false, m: false },
    save: { raw: "None", kinds: ["none"] },
  };
  return {
    id: "x",
    name: "X",
    class: "Wizard",
    level: 1,
    school: "",
    sphere: "",
    casting_time: "1",
    range: "60 yds.",
    duration: "8 rds. + 1d8 rds.",
    area_of_effect: "1d4 persons",
    components: "V",
    saving_throw: "None",
    description: "",
    stats,
    caster_level: 15,
    values,
  };
}

describe("valueFormula", () => {
  it("puts the served dice first, then the served flat amount", () => {
    expect(valueFormula({ amount: 8, dice: "1d8", unit: "round" })).toBe("1d8+8");
    expect(valueFormula({ amount: -1, dice: "2d4" })).toBe("2d4-1");
    expect(valueFormula({ amount: 0, dice: "1d4 + 1d6" })).toBe("1d4+1d6");
  });

  it("has no formula without dice or for a range of values", () => {
    expect(valueFormula({ amount: 150, unit: "yard" })).toBeNull();
    expect(valueFormula({ amount: 1, upper: 5, dice: "1d4" })).toBeNull();
    expect(valueFormula(undefined)).toBeNull();
  });
});

describe("statDice", () => {
  it("lists the stats whose served value has dice, in the detail grid's order", () => {
    const s = spell({
      caster_level: 15,
      casting_time: { amount: 1 },
      range: { amount: 210, unit: "yard" },
      duration: { amount: 8, dice: "1d8", unit: "round" },
      area: { targets: { amount: 0, dice: "1d4" } },
    });
    expect(statDice(s, unit)).toEqual([
      { key: "duration", value: "8 + 1d8 rd", formula: "1d8+8" },
      { key: "area", value: "1d4 persons", formula: "1d4" },
    ]);
  });

  it("is empty when the server computed no values", () => {
    expect(statDice(spell(undefined), unit)).toEqual([]);
  });
});
