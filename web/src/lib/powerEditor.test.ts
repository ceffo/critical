// The equipment editor's powers (ADR 0033 step 17): the form's drafts to the powers sent and back,
// the armour's and shield's "+N" as the magic power, the ready-made rows.
import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { powersChangeText, powerText } from "./itemPowers";
import {
  draftsToPowers,
  newPowerDraft,
  plusOf,
  powerDrafts,
  powersKey,
  templatePower,
} from "./powerEditor";
import type { ItemPowerDef } from "@/types/items";

const ring: ItemPowerDef = {
  id: "power_1",
  effects: { ac: -2, save_bonus: 2, without_magic_armour: true },
};

describe("ADR 0033 step 17: the +N", () => {
  it("is the magic power of an armour or a shield, ac -N", () => {
    const powers = [{ id: "magic", effects: { ac: -2 } }, ring];
    expect(plusOf(powers)).toBe(2);
    expect(powerDrafts("armour", powers).map((d) => d.id)).toEqual(["power_1"]);
    expect(draftsToPowers("armour", 2, powerDrafts("armour", powers))).toEqual(
      powers,
    );
    expect(draftsToPowers("shield", 0, [])).toEqual([]);
  });

  it("is no field of the other categories: their magic power is a power like any", () => {
    const powers = [{ id: "magic", effects: { ac: -2 } }];
    expect(powerDrafts("magic", powers).map((d) => d.id)).toEqual(["magic"]);
    expect(draftsToPowers("magic", 3, powerDrafts("magic", powers))).toEqual(
      powers,
    );
  });
});

describe("ADR 0033 step 17: the drafts", () => {
  it("send a power's effects, trigger and conditions, a new one without an id", () => {
    const d = {
      ...newPowerDraft(),
      minStr: "18",
      effects: [
        { key: "weapon_buff" as const, value: "5" },
        { key: "multiplier" as const, value: "2" },
      ],
      weaponKind: "melee" as const,
    };
    expect(draftsToPowers("magic", 0, [d])).toEqual([
      {
        id: "",
        minStr: 18,
        effects: { weapon_buff: 5, multiplier: 2, weapon_kind: "melee" },
      },
    ]);
  });

  it("drop a power with no effect left, and refuse a value that is not a number", () => {
    expect(
      draftsToPowers("gear", 0, [{ ...newPowerDraft(), effects: [] }]),
    ).toEqual([]);
    expect(
      draftsToPowers("gear", 0, [
        { ...newPowerDraft(), effects: [{ key: "ac", value: "x" }] },
      ]),
    ).toBeNull();
    expect(
      draftsToPowers("gear", 0, [{ ...newPowerDraft(), minStr: "18.5" }]),
    ).toBeNull();
  });

  it("round-trip a stored power, so an untouched form changes nothing", () => {
    const sent = draftsToPowers("magic", 0, powerDrafts("magic", [ring]));
    expect(sent).not.toBeNull();
    expect(powersKey(sent!)).toBe(powersKey([ring]));
  });
});

describe("ADR 0033 step 17: the templates", () => {
  it("give the common protection items as effect codes", () => {
    const sent = (t: Parameters<typeof templatePower>[0], n: number) =>
      draftsToPowers("magic", 0, [templatePower(t, n)])?.[0].effects;
    expect(sent("ring_of_protection", 2)).toEqual({
      ac: -2,
      save_bonus: 2,
      without_magic_armour: true,
    });
    expect(sent("cloak_of_protection", 1)).toEqual({ ac: -1, save_bonus: 1 });
    expect(sent("bracers_of_defense", 4)).toEqual({ base_ac: 4 });
  });
});

describe("ADR 0033 step 17: a power in words", () => {
  for (const lang of ["en", "fr"] as const) {
    it(`words the effects, conditions and the +N (${lang})`, async () => {
      const t = await i18next.createInstance().init({
        lng: lang,
        defaultNS: "common",
        resources,
        interpolation: { escapeValue: false },
      });
      expect(powerText(t, ring)).toBe(
        lang === "en"
          ? "Saves +2 · AC −2 (without magic armour)"
          : "Sauvegardes +2 · CA −2 (sans armure magique)",
      );
      expect(powersChangeText(t, '[{"id":"magic","effects":{"ac":-1}}]')).toBe(
        lang === "en" ? "magic +1" : "magie +1",
      );
      expect(powersChangeText(t, "[]")).toBe(lang === "en" ? "none" : "aucun");
    });
  }
});
