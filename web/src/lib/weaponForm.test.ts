import { describe, expect, it } from "vitest";
import {
  ammoKindSuggestions,
  changedWeaponFields,
  draftFromWeapon,
  emptyWeaponDraft,
  weaponDraftToBody,
} from "./weaponForm";
import type { Weapon } from "@/types/combat";

const bastard: Weapon = {
  id: "bastard_sword",
  name: "Bastard Sword +2",
  damage_sm: "2d4",
  damage_l: "2d8",
  magic_to_hit: 2,
  magic_damage: 2,
  kind: "melee",
  two_handed: true,
  ammo_kind: "",
};

describe("weaponDraftToBody", () => {
  it("turns the draft into the API body", () => {
    const d = {
      ...draftFromWeapon(bastard),
      magic_to_hit: "3",
      magic_damage: " +4 ",
    };
    const { body, errors } = weaponDraftToBody(d);
    expect(errors).toEqual({});
    expect(body).toEqual({
      name: "Bastard Sword +2",
      damage_sm: "2d4",
      damage_l: "2d8",
      magic_to_hit: 3,
      magic_damage: 4,
      kind: "melee",
      two_handed: true,
      ammo_kind: "",
    });
  });

  it("flags text that is not a whole number, and treats blank bonuses as 0", () => {
    const { body, errors } = weaponDraftToBody({
      ...emptyWeaponDraft,
      magic_to_hit: "1.5",
      magic_damage: "",
    });
    expect(errors).toEqual({ magic_to_hit: "notANumber" });
    expect(body.magic_damage).toBe(0);
  });

  it("sends the ammo kind of a missile weapon only", () => {
    const bow = {
      ...emptyWeaponDraft,
      kind: "missile" as const,
      ammo_kind: "arrow",
    };
    expect(weaponDraftToBody(bow).body.ammo_kind).toBe("arrow");
    expect(weaponDraftToBody({ ...bow, kind: "melee" }).body.ammo_kind).toBe(
      "",
    );
  });

  it("leaves the dice to the server (invalid dice pass through)", () => {
    const { body, errors } = weaponDraftToBody({
      ...emptyWeaponDraft,
      damage_sm: "2x6",
    });
    expect(errors).toEqual({});
    expect(body.damage_sm).toBe("2x6");
  });
});

describe("changedWeaponFields", () => {
  it("keeps only what differs", () => {
    const { body } = weaponDraftToBody({
      ...draftFromWeapon(bastard),
      magic_to_hit: "3",
    });
    expect(changedWeaponFields(bastard, body)).toEqual({ magic_to_hit: 3 });
  });

  it("is empty when nothing changed", () => {
    const { body } = weaponDraftToBody(draftFromWeapon(bastard));
    expect(changedWeaponFields(bastard, body)).toEqual({});
  });
});

describe("ammoKindSuggestions", () => {
  it("lists arrow and bolt plus the kinds in use, once each", () => {
    expect(
      ammoKindSuggestions(
        [{ ...bastard, ammo_kind: "stone" }],
        ["arrow", "", "dart"],
      ),
    ).toEqual(["arrow", "bolt", "dart", "stone"]);
  });
});
