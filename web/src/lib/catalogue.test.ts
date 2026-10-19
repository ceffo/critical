import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { catalogueRows, fold, pickBody, type CopyDraft } from "./catalogue";
import type { EquipmentCatalogue } from "@/types/items";

const catalogue: EquipmentCatalogue = {
  weapons: [
    {
      code: "long_sword",
      name: { en: "Long sword", fr: "Épée longue" },
      weaponType: "long_sword",
      weaponKind: "melee",
      size: "m",
      damageTypes: ["s"],
      speed: 5,
      damageSm: "1d8",
      damageL: "1d12",
      twoHanded: false,
      fires: [],
    },
    {
      code: "long_bow",
      name: { en: "Long bow", fr: "Arc long" },
      weaponType: "long_bow",
      weaponKind: "missile",
      size: "l",
      damageTypes: [],
      speed: 8,
      damageSm: "",
      damageL: "",
      twoHanded: true,
      fires: ["arrow"],
    },
  ],
  ammunition: [],
  armour: [
    {
      code: "chain_mail",
      name: { en: "Chain mail", fr: "Cotte de mailles" },
      baseAc: 5,
      skillColumn: "metal",
      armorRow: "chain",
    },
  ],
  shields: [],
};

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

const copy = (over: Partial<CopyDraft> = {}): CopyDraft => ({
  name: "Épée longue",
  magic: 0,
  count: "1",
  equip: true,
  ...over,
});

describe("catalogueRows", () => {
  it("searches the names in the player's language, accents and case ignored", async () => {
    const t = await makeT("fr");
    const rows = catalogueRows(catalogue, "weapon", "EPEE", "fr", t);
    expect(rows.map((r) => r.code)).toEqual(["long_sword"]);
    expect(rows[0].cells).toEqual(["1d8 · G 1d12", "5", "Mêlée"]);
    expect(
      catalogueRows(catalogue, "weapon", "sword", "fr", t).map((r) => r.code),
    ).toEqual([]);
  });

  it("shows a launcher's missing dice as a dash, and an armour's codes as words", async () => {
    const t = await makeT("en");
    expect(catalogueRows(catalogue, "weapon", "bow", "en", t)[0].cells[0]).toBe(
      "—",
    );
    expect(catalogueRows(catalogue, "armour", "", "en", t)[0].cells).toEqual([
      "5",
      "Metal (bard)",
      "Chain mail",
    ]);
  });

  it("folds", () => expect(fold(" Épée ")).toBe("epee"));
});

describe("pickBody", () => {
  const sword = {
    code: "long_sword",
    name: catalogue.weapons[0].name,
    cells: [],
  };
  it("sends the code, the +N as to hit and damage, equip; the row's name is not sent", () => {
    expect(pickBody(sword, "weapon", copy({ magic: 2 }), "fr")).toEqual({
      catalogueCode: "long_sword",
      magicToHit: 2,
      magicDamage: 2,
      equip: true,
    });
  });
  it("sends a renamed copy in the UI language only", () => {
    expect(
      pickBody(
        sword,
        "weapon",
        copy({ name: "Lame de Kelsen", equip: false }),
        "fr-CA",
      ),
    ).toEqual({ catalogueCode: "long_sword", name: { fr: "Lame de Kelsen" } });
  });
  it("sends an ammunition's count and bonus, never equip; null for a count below 1 or no number", () => {
    const arrows = {
      code: "sheaf_arrow",
      name: { en: "Sheaf arrow" },
      cells: [],
    };
    expect(
      pickBody(
        arrows,
        "ammunition",
        copy({ name: "Sheaf arrow", magic: 1, count: "12" }),
        "en",
      ),
    ).toEqual({ catalogueCode: "sheaf_arrow", magicalBonus: 1, count: 12 });
    for (const count of ["x", "", "0", "-2", "1.5"])
      expect(
        pickBody(arrows, "ammunition", copy({ count }), "en"),
      ).toBeNull();
  });
  it("gives armour no magic (its +N is a power)", () => {
    const mail = {
      code: "chain_mail",
      name: catalogue.armour[0].name,
      cells: [],
    };
    expect(
      pickBody(mail, "armour", copy({ name: "Chain mail", magic: 3 }), "en"),
    ).toEqual({ catalogueCode: "chain_mail", equip: true });
  });
});
