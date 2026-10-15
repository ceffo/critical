import { beforeEach, describe, expect, it, vi } from "vitest";
import type { LogEntry, LogKind } from "@/types/log";

// A fake server action stack: what GET /api/log, POST /api/log/undo, POST /api/log/{id}/void and
// the initiative reroll do to it (the rules themselves are Go tests). It lets the store be driven
// through its real refresh path, as two clients would see the server.
const server = vi.hoisted(() => {
  type Row = { id: number; kind: string; voided: boolean };
  const s = {
    rows: [] as Row[],
    nextId: 1,
    calls: [] as string[],
    push(kind: string) {
      s.rows.push({ id: s.nextId++, kind, voided: false });
    },
    top(): Row | undefined {
      return [...s.rows].reverse().find((r) => !r.voided);
    },
    pop(expected: number) {
      const top = s.top();
      if (!top) throw new Error("nothing to undo: expected #" + expected);
      if (top.id !== expected) throw new Error(`the newest entry changed: #${top.id}, not #${expected}`);
      top.voided = true;
    },
  };
  return s;
});

vi.mock("@/api/log", () => ({
  fetchLog: async () => ({
    entries: [...server.rows].reverse().map(
      (r) =>
        ({
          id: r.id,
          kind: r.kind as LogKind,
          status: r.voided ? "voided" : "active",
          payload: {},
          effectivePayload: {},
        }) as LogEntry,
    ),
    topId: server.top()?.id ?? null,
  }),
  undoLogEntry: async (expected: number) => {
    server.calls.push(`undo ${expected}`);
    server.pop(expected);
  },
  voidLogEntry: async (id: number) => {
    server.calls.push(`void ${id}`);
    server.pop(id);
  },
  amendLogEntry: async () => undefined,
}));
vi.mock("../api/round", () => ({
  fetchRound: async () => ({ round: 1 }),
  // The reroll pops the newest initiative and records a new one (ADR 0013 step 4).
  rerollInitiative: async () => {
    server.calls.push("reroll");
    const top = server.top();
    if (top?.kind !== "initiative") throw new Error("not the latest entry");
    top.voided = true;
    server.push("initiative");
    return { round: 1, d10: 5 };
  },
}));
vi.mock("@/api/hp", () => ({
  getHpState: async () => ({ currentHp: 1, maxHp: 1, events: [] }),
}));
vi.mock("@/api/items", () => ({ fetchItems: async () => [] }));
vi.mock("@/api/combat", () => ({ fetchWeapons: async () => [] }));
vi.mock("../api/buffs", () => ({
  fetchActiveBuffs: async () => [],
  fetchSavedBuffs: async () => [],
  fetchStrength: async () => null,
}));

import { useLogStore } from "./log";
import { useTableStore } from "./table";
import { useRollStore } from "./roll";
import type { RollResult } from "@/types/combat";

const topId = () => useLogStore.getState().topId;

describe("global undo against the server stack", () => {
  beforeEach(async () => {
    server.rows = [];
    server.nextId = 1;
    server.calls = [];
    useTableStore.setState({ error: null, busy: false });
    await useTableStore.getState().refresh();
  });

  // Checklist 1: an empty log, fresh or after popping everything: no request, nothing happens.
  it("empty log: topId null and undo sends nothing", async () => {
    expect(topId()).toBeNull();
    expect(await useTableStore.getState().undo()).toBe(false);

    server.push("hp_change");
    await useTableStore.getState().refresh();
    expect(await useTableStore.getState().undo()).toBe(true);
    expect(topId()).toBeNull();
    expect(await useTableStore.getState().undo()).toBe(false);
    expect(server.calls).toEqual(["undo 1"]);
    expect(useTableStore.getState().error).toBeNull();
  });

  // Checklist 2: client A pops while B still shows the old top; B's undo is refused with the
  // server's error, B refreshes, and its next undo pops the new top.
  it("stale top from another client: error shown, refreshed, next undo pops the new top", async () => {
    server.push("attack");
    server.push("hp_change");
    await useTableStore.getState().refresh();
    expect(topId()).toBe(2);

    server.pop(2); // client A
    expect(await useTableStore.getState().undo()).toBe(false);
    expect(useTableStore.getState().error).toMatch(/newest entry changed/);
    expect(topId()).toBe(1);
    expect(server.top()?.id).toBe(1);

    expect(await useTableStore.getState().undo()).toBe(true);
    expect(server.calls).toEqual(["undo 2", "undo 1"]);
    expect(topId()).toBeNull();
  });

  // Checklist 3: initiative reroll and the row Void after a global undo: the topId the row Void
  // and the button use follows the server after every action.
  it("reroll and row Void after an undo keep topId in step with the server", async () => {
    server.push("round_advance");
    server.push("initiative");
    server.push("hp_change");
    await useTableStore.getState().refresh();

    await useTableStore.getState().undo(); // pops the damage
    expect(topId()).toBe(2);
    expect(await useTableStore.getState().rerollInitiative()).toBe(true);
    expect(topId()).toBe(4);
    expect(topId()).toBe(server.top()?.id);

    // The row Void targets topId (CombatLog), then the global undo the next top.
    expect(await useTableStore.getState().voidEntry(topId()!)).toBe(true);
    expect(topId()).toBe(1);
    expect(await useTableStore.getState().undo()).toBe(true);
    expect(server.calls).toEqual(["undo 3", "reroll", "void 4", "undo 1"]);
    expect(topId()).toBeNull();
    expect(useTableStore.getState().error).toBeNull();
  });

  // ADR 0010 step 6: popping a correction gives the attack its previous payload; the card must
  // read it from the refreshed log, not keep this session's corrected roll response.
  it("a pop drops this session's live roll so the card reads the log", async () => {
    server.push("attack");
    server.push("correction");
    await useTableStore.getState().refresh();
    useRollStore.setState({ last: { kind: "attack", result: { logEntryId: 1 } as RollResult } });

    expect(await useTableStore.getState().undo()).toBe(true);
    expect(useRollStore.getState().last).toBeNull();
    expect(topId()).toBe(1);

    useRollStore.setState({ last: { kind: "attack", result: { logEntryId: 1 } as RollResult } });
    expect(await useTableStore.getState().voidEntry(1)).toBe(true);
    expect(useRollStore.getState().last).toBeNull();
  });
});
