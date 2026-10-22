import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SpellSlot } from "@/types/spells";

const h = vi.hoisted(() => ({
  slots: [] as unknown[],
  prepared: [] as { spells: string[]; cls?: string }[],
  infoFor: [] as (string | undefined)[],
}));

vi.mock("../api/spells", () => ({
  fetchSpellSlots: async () => h.slots,
  prepareSpells: async (spells: string[], cls?: string) => {
    h.prepared.push({ spells, cls });
    return h.slots;
  },
  fetchPreparationInfo: async (cls?: string) => {
    h.infoFor.push(cls);
    return { spellbook: [], limits: [] };
  },
}));

import { useSpellStore, validEntry } from "./spells";

const slot = (cls?: string) =>
  ({ level: 1, class: cls }) as unknown as SpellSlot;

beforeEach(() => {
  h.prepared.length = 0;
  h.infoFor.length = 0;
  useSpellStore.setState({ slots: [], entry: "", infoLoaded: false });
});

describe("the casting-entry picker", () => {
  it("keeps a pick the slots have and drops one they do not", () => {
    expect(validEntry([slot("mage"), slot("cleric")], "mage")).toBe("mage");
    expect(validEntry([slot("mage"), slot("cleric")], "bard")).toBe("");
    expect(validEntry([slot()], "mage")).toBe("");
    expect(validEntry([slot("mage")], "")).toBe("");
  });

  it("picks an entry and reads its spellbook", async () => {
    await useSpellStore.getState().pickEntry("cleric");
    expect(useSpellStore.getState().entry).toBe("cleric");
    expect(useSpellStore.getState().infoLoaded).toBe(true);
    expect(h.infoFor).toEqual(["cleric"]);
  });

  it("prepares the picked entry of a two-caster", async () => {
    h.slots = [slot("mage"), slot("cleric")];
    useSpellStore.setState({ slots: h.slots as SpellSlot[], entry: "cleric" });
    await useSpellStore.getState().prepare(["a"]);
    expect(h.prepared).toEqual([{ spells: ["a"], cls: "cleric" }]);
  });

  it("sends no class for a single-class caster after a pick made on another character", async () => {
    useSpellStore.setState({ entry: "mage" });
    h.slots = [slot()];
    await useSpellStore.getState().fetchSlots(); // the other character's slots
    expect(useSpellStore.getState().entry).toBe("");
    expect(useSpellStore.getState().infoLoaded).toBe(false);
    await useSpellStore.getState().prepare(["a"]);
    expect(h.prepared).toEqual([{ spells: ["a"], cls: undefined }]);
  });

  it("does not send a stale pick even before the slots are read again", async () => {
    h.slots = [slot()];
    useSpellStore.setState({ slots: h.slots as SpellSlot[], entry: "mage" });
    await useSpellStore.getState().prepare(["a"]);
    await useSpellStore.getState().fetchInfo();
    expect(h.prepared[0].cls).toBeUndefined();
    expect(h.infoFor).toEqual([undefined]);
  });
});
