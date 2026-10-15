import { describe, expect, it } from "vitest";
import type { SpellbookEntry, SpellSlot, SpendableItem } from "@/types/spells";
import { planCast } from "./castInputs";

const book: SpellbookEntry[] = [
  { id: "alarm", name: "Alarm", level: 1 },
  {
    id: "chromatic_orb",
    name: "Chromatic Orb",
    level: 1,
    consume_item_type: "gem",
    attack_roll: true,
  },
  {
    id: "rarys_mnemonic_enhancer",
    name: "Rary's Mnemonic Enhancer",
    level: 4,
    slot_splits: [[1, 1, 1], [1, 2], [3]],
  },
];
const diamonds: SpendableItem = { id: "diamond", name: "Diamond", count: 3 };
const rubies: SpendableItem = { id: "ruby", name: "Ruby", count: 2 };
const withItems = (...items: SpendableItem[]): SpellbookEntry[] =>
  book.map((e) => (e.id === "chromatic_orb" ? { ...e, items } : e));
const slot = (id: string, level = 1): SpellSlot => ({
  level,
  prepared_spell_id: id,
  prepared_spell_name: id,
  used: false,
});
const blank: SpellSlot = {
  level: 1,
  prepared_spell_id: "",
  prepared_spell_name: "",
  used: false,
  extra: true,
};

describe("planCast", () => {
  it("an attack-roll spell needs a target AC: blocked without one, nothing sent", () => {
    const p = planCast(slot("chromatic_orb"), false, withItems(diamonds), {});
    expect(p.needs?.attack_roll).toBe(true);
    expect(p.blocked).toBe(true);
    const ok = planCast(slot("chromatic_orb"), false, withItems(diamonds), {
      acText: " -2 ",
    });
    expect(ok.blocked).toBe(false);
    expect(ok.options).toEqual({
      spellId: undefined,
      extraSlots: undefined,
      targetAc: -2,
      itemId: undefined,
    });
  });

  it.each([
    ["no gem served: the server refuses", [], 0, undefined],
    ["one gem: no select, the server takes it", [diamonds], 1, undefined],
    [
      "two gems: a select, the first by default",
      [diamonds, rubies],
      2,
      "diamond",
    ],
  ])("%s", (_, items, shown, sent) => {
    const p = planCast(slot("chromatic_orb"), false, withItems(...items), {
      acText: "4",
    });
    expect(p.spendable).toEqual(items);
    expect(p.spendable).toHaveLength(shown);
    expect(p.options.itemId).toBe(sent);
  });

  it("sends the item picked among several", () => {
    const p = planCast(
      slot("chromatic_orb"),
      false,
      withItems(diamonds, rubies),
      { acText: "4", itemPick: "ruby" },
    );
    expect(p.options.itemId).toBe("ruby");
  });

  it("a blank extra slot asks for the picked spell's inputs (Chromatic Orb: target AC)", () => {
    expect(planCast(blank, true, withItems(diamonds), {}).blocked).toBe(true);
    const p = planCast(blank, true, withItems(diamonds), {
      pick: "chromatic_orb",
    });
    expect(p.needs?.id).toBe("chromatic_orb");
    expect(p.blocked).toBe(true);
    const ok = planCast(blank, true, withItems(diamonds), {
      pick: "chromatic_orb",
      acText: "5",
    });
    expect(ok.blocked).toBe(false);
    expect(ok.options).toMatchObject({ spellId: "chromatic_orb", targetAc: 5 });
  });

  it("a spell that grants slots offers its splits, the first by default", () => {
    const p = planCast(slot("rarys_mnemonic_enhancer", 4), false, book, {});
    expect(p.slotSplits).toEqual([[1, 1, 1], [1, 2], [3]]);
    expect(p.options.extraSlots).toEqual([1, 1, 1]);
    expect(
      planCast(slot("rarys_mnemonic_enhancer", 4), false, book, {
        split: "1,2",
      }).options.extraSlots,
    ).toEqual([1, 2]);
    expect(
      planCast(slot("rarys_mnemonic_enhancer", 4), false, book, { split: "3" })
        .options.extraSlots,
    ).toEqual([3]);
  });

  it("a plain spell asks nothing", () => {
    const p = planCast(slot("alarm"), false, book, {});
    expect(p.blocked).toBe(false);
    expect(p.options).toEqual({
      spellId: undefined,
      extraSlots: undefined,
      targetAc: undefined,
      itemId: undefined,
    });
  });

  it("a spell with cast-time modes offers them, the first by default, and sends the one picked", () => {
    const sheet = {
      caster_level: 15,
      dice: { groups: [{ count: 3, sides: 10 }], modifier: 0 },
      instances: { groups: [], modifier: 1 },
      rounds: { groups: [], modifier: 1 },
      type: "cold",
    };
    const modes = [
      { key: "ice_plane" },
      { key: "hemisphere" },
      { key: "ice_sheet", damage: sheet },
    ];
    const withModes: SpellbookEntry[] = [
      ...book,
      { id: "wall_of_ice", name: "Wall of Ice", level: 4, modes },
    ];
    const p = planCast(slot("wall_of_ice", 4), false, withModes, {});
    expect(p.modes).toEqual(modes);
    expect(p.mode?.key).toBe("ice_plane");
    expect(p.options.mode).toBe("ice_plane");
    const picked = planCast(slot("wall_of_ice", 4), false, withModes, {
      mode: "ice_sheet",
    });
    expect(picked.mode?.damage).toEqual(sheet);
    expect(picked.options.mode).toBe("ice_sheet");
    expect(
      planCast(slot("alarm"), false, withModes, {}).options.mode,
    ).toBeUndefined();
  });
});
