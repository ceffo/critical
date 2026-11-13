import { describe, expect, it } from "vitest";
import { testI18n } from "@/test/render";
import type { EquipmentCatalogue } from "@/types/items";
import {
  groupWeaponRows,
  addWeapon,
  isSpecialization,
  removeWeapon,
  specialize,
  weaponRows,
} from "./weapons";

const fighter = [
  { code: "long_bow", specializationSlots: 3 },
  { code: "long_sword", specializationSlots: 2 },
];

describe("weaponRows", () => {
  it("lists the class's types in the options' order, then a kept pick outside them", () => {
    const rows = weaponRows(
      [{ code: "dagger", specializationSlots: 0 }],
      [
        { code: "long_sword", slots: 1 },
        { code: "dagger", slots: 1 },
      ],
      [
        { code: "long_sword", slots: 1, allowed: false },
        { code: "dagger", slots: 1, allowed: true },
      ],
    );
    expect(rows.map((r) => [r.code, r.allowed, r.pick?.slots])).toEqual([
      ["dagger", true, 1],
      ["long_sword", false, 1],
    ]);
  });

  it("reads allowed from the server's figures when it has them", () => {
    const rows = weaponRows(
      fighter,
      [],
      [{ code: "long_bow", slots: 1, allowed: false }],
    );
    expect(rows[0].allowed).toBe(false);
    expect(rows[1].allowed).toBe(true);
  });
});

describe("the pick edits", () => {
  it("adds a plain pick once and removes it", () => {
    const one = addWeapon([], "long_sword");
    expect(one).toEqual([{ code: "long_sword", slots: 1 }]);
    expect(addWeapon(one, "long_sword")).toBe(one);
    expect(removeWeapon(one, "long_sword")).toEqual([]);
  });

  it("specializes at the options' cost and goes back to a plain pick", () => {
    const picked = [{ code: "long_bow", slots: 1 }];
    const spec = specialize(picked, "long_bow", 3);
    expect(spec).toEqual([{ code: "long_bow", slots: 3 }]);
    expect(isSpecialization(spec[0])).toBe(true);
    expect(specialize(spec, "long_bow", null)).toEqual(picked);
    // A type not yet picked is added as the specialization.
    expect(specialize([], "long_sword", 2)).toEqual([
      { code: "long_sword", slots: 2 },
    ]);
    expect(isSpecialization(picked[0])).toBe(false);
    expect(isSpecialization(undefined)).toBe(false);
  });
});

describe("groupWeaponRows", () => {
  const w = (code: string, fr: string, weaponKind: string) => ({
    code,
    name: { fr, en: fr },
    weaponKind,
  });
  const catalogue = {
    weapons: [
      w("long_bow", "Arc long", "missile"),
      w("quarterstaff", "Bâton", "melee"),
      w("heavy_horse_lance", "Lance d’arçon lourde", "melee"),
      w("dart", "Fléchette", "missile"),
    ],
    ammunition: [],
    armour: [],
    shields: [],
  } as unknown as EquipmentCatalogue;
  const rows = weaponRows(
    ["long_bow", "quarterstaff", "heavy_horse_lance", "dart"].map((code) => ({
      code,
      specializationSlots: 0,
    })),
    [{ code: "homebrew_thing", slots: 1 }],
    undefined,
  );

  it("groups by the catalogue's family, sorted by name in the language, a code it lacks last and heading-less", async () => {
    const { t } = await testI18n("fr");
    const groups = groupWeaponRows(rows, catalogue, "fr", t);
    expect(groups.map((g) => [g.key, g.rows.map((r) => r.code)])).toEqual([
      ["melee", ["quarterstaff", "heavy_horse_lance"]],
      ["missile", ["long_bow", "dart"]],
      ["", ["homebrew_thing"]],
    ]);
  });

  it("is one heading-less group, in the options' order, until the catalogue is read", async () => {
    const { t } = await testI18n("fr");
    const groups = groupWeaponRows(rows, null, "fr", t);
    expect(groups).toHaveLength(1);
    expect(groups[0].key).toBe("");
    expect(groups[0].rows.map((r) => r.code)).toEqual([
      "long_bow",
      "quarterstaff",
      "heavy_horse_lance",
      "dart",
      "homebrew_thing",
    ]);
  });
});
