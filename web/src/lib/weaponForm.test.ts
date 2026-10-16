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
  name: { en: "Bastard Sword +2", fr: "Épée bâtarde +2" },
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
      ...draftFromWeapon(bastard, "en"),
      magic_to_hit: "3",
      magic_damage: " +4 ",
    };
    const { body, errors } = weaponDraftToBody(d, "en");
    expect(errors).toEqual({});
    expect(body).toEqual({
      name: { en: "Bastard Sword +2" },
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
    const { body, errors } = weaponDraftToBody(
      {
        ...emptyWeaponDraft,
        magic_to_hit: "1.5",
        magic_damage: "",
      },
      "en",
    );
    expect(errors).toEqual({ magic_to_hit: "notANumber" });
    expect(body.magic_damage).toBe(0);
  });

  it("sends the ammo kind of a missile weapon only", () => {
    const bow = {
      ...emptyWeaponDraft,
      kind: "missile" as const,
      ammo_kind: "arrow",
    };
    expect(weaponDraftToBody(bow, "en").body.ammo_kind).toBe("arrow");
    expect(
      weaponDraftToBody({ ...bow, kind: "melee" }, "en").body.ammo_kind,
    ).toBe("");
  });

  it("leaves the dice to the server (invalid dice pass through)", () => {
    const { body, errors } = weaponDraftToBody(
      {
        ...emptyWeaponDraft,
        damage_sm: "2x6",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body.damage_sm).toBe("2x6");
  });
});

describe("changedWeaponFields", () => {
  it("keeps only what differs", () => {
    const { body } = weaponDraftToBody(
      {
        ...draftFromWeapon(bastard, "en"),
        magic_to_hit: "3",
      },
      "en",
    );
    expect(changedWeaponFields(bastard, body, "en")).toEqual({
      magic_to_hit: 3,
    });
  });

  it("is empty when nothing changed", () => {
    const { body } = weaponDraftToBody(draftFromWeapon(bastard, "en"), "en");
    expect(changedWeaponFields(bastard, body, "en")).toEqual({});
  });

  it("sends the name in the UI language only (ADR 0023 step 11)", () => {
    const d = draftFromWeapon(bastard, "fr");
    expect(d.name).toBe("Épée bâtarde +2");
    const { body } = weaponDraftToBody({ ...d, name: "Épée bâtarde" }, "fr");
    expect(changedWeaponFields(bastard, body, "fr")).toEqual({
      name: { fr: "Épée bâtarde" },
    });
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
