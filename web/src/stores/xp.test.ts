import { beforeEach, describe, expect, it, vi } from "vitest";
import type { XpLedger } from "@/types/xp";

// Counts the XP and log reads: an XP write answers with the ledger and refreshes the journal
// (its entry is the new top, ADR 0015 step 4) without reading the ledger again; an undo re-reads
// it, since the pop may have been an XP award. Both re-read the character (its level follows the
// XP, ADR 0030 step 3); another action does not (GET /api/character is the database probe).
const calls = vi.hoisted(() => ({
  fetchXp: 0,
  fetchLog: 0,
  undo: 0,
  character: 0,
}));
const ledger = (total: number) =>
  ({ total, entries: [] }) as unknown as XpLedger;

vi.mock("@/api/xp", () => ({
  fetchXp: async () => {
    calls.fetchXp++;
    return ledger(100);
  },
  postXpEntry: async () => ledger(200),
  patchXpEntry: async () => ledger(200),
  deleteXpEntry: async () => ledger(200),
}));
vi.mock("@/api/log", () => ({
  fetchLog: async () => {
    calls.fetchLog++;
    return { entries: [], topId: 7, redo: null };
  },
  undoLogEntry: async () => {
    calls.undo++;
  },
}));
vi.mock("@/api/character", () => ({
  fetchCharacter: async () => {
    calls.character++;
    return {};
  },
}));
vi.mock("../api/round", () => ({ fetchRound: async () => ({ round: 1 }) }));
vi.mock("@/api/hp", () => ({
  getHpState: async () => ({ currentHp: 1, maxHp: 1, events: [] }),
}));
vi.mock("@/api/items", () => ({ fetchItems: async () => [] }));
vi.mock("@/api/combat", () => ({ fetchWeapons: async () => [] }));
vi.mock("@/api/ac", () => ({ fetchAc: async () => null }));
vi.mock("../api/buffs", () => ({
  fetchActiveBuffs: async () => [],
  fetchSavedBuffs: async () => [],
  fetchStrength: async () => null,
}));

import { useXpStore } from "./xp";
import { useTableStore } from "./table";

describe("XP writes and the journal", () => {
  beforeEach(async () => {
    await useXpStore.getState().fetchXp();
    await useTableStore.getState().refresh();
    calls.fetchXp = calls.fetchLog = calls.undo = calls.character = 0;
  });

  it("a write refreshes the journal but does not read the ledger again", async () => {
    expect(
      await useXpStore.getState().add({ base: 100, bonus: false }),
    ).toBeNull();
    expect(await useXpStore.getState().update(1, { base: 50 })).toBeNull();
    expect(await useXpStore.getState().remove(1)).toBeNull();
    expect(calls).toEqual({ fetchXp: 0, fetchLog: 3, undo: 0, character: 3 });
    expect(useXpStore.getState().ledger?.total).toBe(200);
  });

  it("an undo re-reads the ledger", async () => {
    await useXpStore.getState().add({ base: 100, bonus: false });
    expect(await useTableStore.getState().undo()).toBe(true);
    expect(calls.undo).toBe(1);
    expect(calls.fetchXp).toBe(1);
    expect(calls.character).toBe(2);
    expect(useXpStore.getState().ledger?.total).toBe(100);
  });

  it("another action does not re-read the character", async () => {
    expect(await useTableStore.getState().run(async () => undefined)).toBe(
      true,
    );
    expect(calls.fetchLog).toBe(1);
    expect(calls.character).toBe(0);
  });
});
