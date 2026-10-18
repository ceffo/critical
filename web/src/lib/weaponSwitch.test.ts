import { describe, expect, it } from "vitest";
import type { Weapon } from "@/types/combat";
import { setModifiers, weaponSwitch } from "./weaponSwitch";

const weapon = (id: string): Weapon => ({
  id,
  name: { en: id },
  damage_sm: "1d6",
  damage_l: "1d8",
  magic_to_hit: 0,
  magic_damage: 0,
  kind: "melee",
  two_handed: false,
  ammo_kind: "",
  weapon_type: "long_sword",
});
const WEAPONS = ["sword", "bow", "dagger", "mace"].map(weapon);
const ids = (ws: Weapon[]) => ws.map((w) => w.id);

describe("weaponSwitch", () => {
  it("offers the previous weapon and lists the rest in the served order", () => {
    const { previous, others } = weaponSwitch(WEAPONS, {
      weaponId: "bow",
      previousWeaponId: "mace",
    });
    expect(previous?.id).toBe("mace");
    expect(ids(others)).toEqual(["sword", "dagger"]);
  });

  it.each([
    ["no previous weapon", ""],
    ["a previous weapon since deleted", "halberd"],
    ["the previous weapon is the one in hand", "bow"],
  ])("lists every other weapon with %s", (_, previousWeaponId) => {
    const { previous, others } = weaponSwitch(WEAPONS, {
      weaponId: "bow",
      previousWeaponId,
    });
    expect(previous).toBeNull();
    expect(ids(others)).toEqual(["sword", "dagger", "mace"]);
  });

  it("has nothing to offer with a single weapon", () => {
    expect(
      weaponSwitch([weapon("sword")], {
        weaponId: "sword",
        previousWeaponId: "",
      }),
    ).toEqual({ previous: null, others: [] });
  });
});

describe("setModifiers", () => {
  it("is empty at the defaults", () => {
    expect(setModifiers({ situational: { value: 0, label: "x" } })).toEqual({
      situational: null,
    });
  });

  it("names a situational modifier other than 0", () => {
    expect(
      setModifiers({
        situational: { value: -2, label: "cover" },
      }),
    ).toEqual({ situational: { value: -2, label: "cover" } });
  });
});
