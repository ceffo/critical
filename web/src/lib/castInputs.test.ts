import { describe, expect, it } from "vitest";
import type { Item } from "@/types/items";
import type { SpellbookEntry, SpellSlot } from "@/types/spells";
import { planCast, spendableItems } from "./castInputs";

const book: SpellbookEntry[] = [
  { id: "alarm", name: "Alarm", level: 1 },
  { id: "chromatic_orb", name: "Chromatic Orb", level: 1, consume_item_type: "gem", attack_roll: true },
  { id: "rarys_mnemonic_enhancer", name: "Rary's Mnemonic Enhancer", level: 4, slot_splits: [[1, 1, 1], [1, 2], [3]] },
];
const item = (id: string, behaviour: Item["behaviour"], count: number, type = "gem") =>
  ({ id, name: id, type, behaviour, count }) as Item;
const powerGem = item("power_gem_dispel", "daily", 1);
const diamonds = item("diamond", "stock", 3);
const rubies = item("ruby", "stock", 2);
const slot = (id: string, level = 1): SpellSlot => ({ level, prepared_spell_id: id, prepared_spell_name: id, used: false });
const blank: SpellSlot = { level: 1, prepared_spell_id: "", prepared_spell_name: "", used: false, extra: true };

describe("spendableItems", () => {
  it("keeps items of the type with some left, never a daily item (the Power Gem)", () => {
    expect(spendableItems([powerGem, diamonds, item("empty", "stock", 0), item("x", "stock", 3, "flask")], "GEM")).toEqual([diamonds]);
    expect(spendableItems([diamonds], undefined)).toEqual([]);
  });
});

describe("planCast", () => {
  it("an attack-roll spell needs a target AC: blocked without one, nothing sent", () => {
    const p = planCast(slot("chromatic_orb"), false, book, [powerGem, diamonds], {});
    expect(p.needs?.attack_roll).toBe(true);
    expect(p.blocked).toBe(true);
    const ok = planCast(slot("chromatic_orb"), false, book, [powerGem, diamonds], { acText: " -2 " });
    expect(ok.blocked).toBe(false);
    expect(ok.options).toEqual({ spellId: undefined, extraSlots: undefined, targetAc: -2, itemId: undefined });
  });

  it.each([
    ["no gem left: the server refuses", [powerGem], 0, undefined],
    ["one gem: no select, the server takes it", [powerGem, diamonds], 1, undefined],
    ["two gems: a select, the first by default", [powerGem, diamonds, rubies], 2, "diamond"],
  ])("%s", (_, items, shown, sent) => {
    const p = planCast(slot("chromatic_orb"), false, book, items, { acText: "4" });
    expect(p.spendable).toHaveLength(shown);
    expect(p.spendable).not.toContain(powerGem);
    expect(p.options.itemId).toBe(sent);
  });

  it("sends the item picked among several", () => {
    const p = planCast(slot("chromatic_orb"), false, book, [diamonds, rubies], { acText: "4", itemPick: "ruby" });
    expect(p.options.itemId).toBe("ruby");
  });

  it("a blank extra slot asks for the picked spell's inputs (Chromatic Orb: target AC)", () => {
    expect(planCast(blank, true, book, [diamonds], {}).blocked).toBe(true);
    const p = planCast(blank, true, book, [diamonds], { pick: "chromatic_orb" });
    expect(p.needs?.id).toBe("chromatic_orb");
    expect(p.blocked).toBe(true);
    const ok = planCast(blank, true, book, [diamonds], { pick: "chromatic_orb", acText: "5" });
    expect(ok.blocked).toBe(false);
    expect(ok.options).toMatchObject({ spellId: "chromatic_orb", targetAc: 5 });
  });

  it("a spell that grants slots offers its splits, the first by default", () => {
    const p = planCast(slot("rarys_mnemonic_enhancer", 4), false, book, [], {});
    expect(p.slotSplits).toEqual([[1, 1, 1], [1, 2], [3]]);
    expect(p.options.extraSlots).toEqual([1, 1, 1]);
    expect(planCast(slot("rarys_mnemonic_enhancer", 4), false, book, [], { split: "1,2" }).options.extraSlots).toEqual([1, 2]);
    expect(planCast(slot("rarys_mnemonic_enhancer", 4), false, book, [], { split: "3" }).options.extraSlots).toEqual([3]);
  });

  it("a plain spell asks nothing", () => {
    const p = planCast(slot("alarm"), false, book, [diamonds], {});
    expect(p.blocked).toBe(false);
    expect(p.options).toEqual({ spellId: undefined, extraSlots: undefined, targetAc: undefined, itemId: undefined });
  });
});
