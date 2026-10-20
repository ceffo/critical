import { describe, expect, it } from "vitest";
import {
  addWeapon,
  isSpecialization,
  removeWeapon,
  specialize,
  weaponRows,
} from "./weapons";

const fighter = [
  { type: "long_bow", specializationSlots: 3 },
  { type: "long_sword", specializationSlots: 2 },
];

describe("weaponRows", () => {
  it("lists the class's types in the options' order, then a kept pick outside them", () => {
    const rows = weaponRows(
      [{ type: "dagger", specializationSlots: 0 }],
      [
        { type: "long_sword", slots: 1 },
        { type: "dagger", slots: 1 },
      ],
      [
        { type: "long_sword", slots: 1, allowed: false },
        { type: "dagger", slots: 1, allowed: true },
      ],
    );
    expect(rows.map((r) => [r.type, r.allowed, r.pick?.slots])).toEqual([
      ["dagger", true, 1],
      ["long_sword", false, 1],
    ]);
  });

  it("reads allowed from the server's figures when it has them", () => {
    const rows = weaponRows(
      fighter,
      [],
      [{ type: "long_bow", slots: 1, allowed: false }],
    );
    expect(rows[0].allowed).toBe(false);
    expect(rows[1].allowed).toBe(true);
  });
});

describe("the pick edits", () => {
  it("adds a plain pick once and removes it", () => {
    const one = addWeapon([], "long_sword");
    expect(one).toEqual([{ type: "long_sword", slots: 1 }]);
    expect(addWeapon(one, "long_sword")).toBe(one);
    expect(removeWeapon(one, "long_sword")).toEqual([]);
  });

  it("specializes at the options' cost and goes back to a plain pick", () => {
    const picked = [{ type: "long_bow", slots: 1 }];
    const spec = specialize(picked, "long_bow", 3);
    expect(spec).toEqual([{ type: "long_bow", slots: 3 }]);
    expect(isSpecialization(spec[0])).toBe(true);
    expect(specialize(spec, "long_bow", null)).toEqual(picked);
    // A type not yet picked is added as the specialization.
    expect(specialize([], "long_sword", 2)).toEqual([
      { type: "long_sword", slots: 2 },
    ]);
    expect(isSpecialization(picked[0])).toBe(false);
    expect(isSpecialization(undefined)).toBe(false);
  });
});
