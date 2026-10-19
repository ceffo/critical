import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { acRowView, shownAc } from "./acView";
import type { AcRow } from "@/types/ac";

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

const names = {
  item: (id: string) =>
    ({ chain: "Cotte de mailles", ring: "Anneau" })[id] ?? id,
  buff: (b: { name: string }) => b.name,
};

describe("acRowView (ADR 0033 step 10, ADR 0035 item 8)", () => {
  it("words each term with its source and its value (fr)", async () => {
    const t = await makeT("fr");
    const rows: AcRow[] = [
      { term: "base", itemId: "chain", value: 5 },
      { term: "armour_magic", value: 0 },
      { term: "shield", value: 0 },
      { term: "dexterity", value: -3, dexterity: 17 },
      { term: "protection", itemId: "ring", value: -2 },
      { term: "buffs", buff: "Protection from Evil", value: -2 },
    ];
    expect(rows.map((r) => acRowView(t, r, names))).toEqual([
      {
        term: "Base",
        source: "Cotte de mailles",
        value: "5",
        missile: null,
        off: null,
      },
      {
        term: "Magie de l'armure",
        source: "aucune",
        value: "—",
        missile: null,
        off: null,
      },
      {
        term: "Bouclier",
        source: "aucun",
        value: "—",
        missile: null,
        off: null,
      },
      {
        term: "Dextérité",
        source: "17",
        value: "−3",
        missile: null,
        off: null,
      },
      {
        term: "Protection",
        source: "Anneau",
        value: "−2",
        missile: null,
        off: null,
      },
      {
        term: "Effets",
        source: "Protection from Evil",
        value: "−2",
        missile: null,
        off: null,
      },
    ]);
  });

  it("words a base with no armour, a body shield's missile value and a row that does not add (en)", async () => {
    const t = await makeT("en");
    expect(acRowView(t, { term: "base", value: 10 }, names).source).toBe(
      "no armour",
    );
    expect(
      acRowView(
        t,
        { term: "shield", itemId: "body", value: -1, missileValue: -2 },
        names,
      ).missile,
    ).toBe("−2 vs missiles");
    expect(
      acRowView(
        t,
        { term: "protection", itemId: "ring", value: -2, off: "magic_armour" },
        names,
      ).off,
    ).toBe("not with magic armour");
  });

  it("shows the override while one is set (ADR 0033 step 13)", () => {
    const base = { ac: -2, floored: false, rows: [], warnings: [] };
    expect(shownAc({ ...base, override: null })).toBe(-2);
    expect(shownAc({ ...base, override: { value: 4, label: "" } })).toBe(4);
  });
});
