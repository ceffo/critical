import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { damageRows, modeName, modeText } from "./spellMode";
import type { SpellDamage } from "@/types/spells";

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

const one = { groups: [], modifier: 1 };
const sheet: SpellDamage = {
  caster_level: 9,
  dice: { groups: [{ count: 3, sides: 10 }], modifier: 0 },
  instances: one,
  rounds: one,
  type: "cold",
};

describe("spell modes", () => {
  it("Wall of Ice in the lookup at caster level 9: one Damage (Ice sheet) row, no plain Damage row", async () => {
    const t = await makeT("en");
    const rows = damageRows(t, {
      id: "wall_of_ice",
      modes: [
        { key: "ice_plane" },
        { key: "hemisphere" },
        { key: "ice_sheet", damage: sheet },
      ],
    });
    expect(rows).toEqual([
      { key: "damage-ice_sheet", label: "Damage (Ice sheet)", damage: sheet },
    ]);
    expect(
      damageRows(await makeT("fr"), {
        id: "wall_of_ice",
        modes: [{ key: "ice_sheet", damage: sheet }],
      })[0].label,
    ).toBe("Dégâts (Plaque de glace)");
  });

  it("a spell without modes keeps its single Damage row; none without damage", async () => {
    const t = await makeT("en");
    expect(damageRows(t, { id: "fireball", damage: sheet })).toEqual([
      { key: "damage", label: "Damage", damage: sheet },
    ]);
    expect(damageRows(t, { id: "alarm" })).toEqual([]);
  });

  it("names a mode and its damage; an untranslated key reads as words", async () => {
    const t = await makeT("en");
    expect(
      modeText(t, "meteor_swarm", {
        key: "small_spheres",
        damage: {
          ...sheet,
          dice: { groups: [{ count: 5, sides: 4 }], modifier: 0 },
          instances: { groups: [], modifier: 8 },
          type: "fire",
        },
      }),
    ).toBe("8 small spheres: 8 × 5d4 fire");
    expect(modeText(t, "ice_storm", { key: "sleet" })).toBe("Sleet: no damage");
    expect(modeName(t, "x_spell", "some_mode")).toBe("some mode");
  });
});
