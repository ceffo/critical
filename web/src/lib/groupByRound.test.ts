import { describe, expect, it } from "vitest";
import type { LogEntry } from "@/types/log";
import { groupByRound, isUndone } from "./groupByRound";

const entry = (id: number, round: number): LogEntry => ({
  id,
  transactionId: id,
  characterId: 1,
  round,
  kind: "hp_change",
  summary: `e${id}`,
  hpDelta: -1,
  effectiveHpDelta: -1,
  note: "",
  effectiveSummary: `e${id}`,
  status: "active",
  payload: {},
  effectivePayload: {},
  snapshots: [],
  corrections: [],
  createdAt: "2026-10-17T20:00:00Z",
});

describe("groupByRound", () => {
  it("keeps server order across round resets (rounds 1,2,3, reset, 1,2)", () => {
    const entries = [
      entry(6, 2),
      entry(5, 1),
      entry(4, 3),
      entry(3, 2),
      entry(2, 1),
      entry(1, 1),
    ];
    const groups = groupByRound(entries, 2);
    expect(groups.map((g) => g.round)).toEqual([2, 1, 3, 2, 1]);
    expect(groups.map((g) => g.entries.map((e) => e.id))).toEqual([
      [6],
      [5],
      [4],
      [3],
      [2, 1],
    ]);
    expect(new Set(groups.map((g) => g.key)).size).toBe(groups.length);
  });

  it("separates out-of-combat round 0 between combats", () => {
    const groups = groupByRound(
      [entry(4, 1), entry(3, 0), entry(2, 0), entry(1, 1)],
      1,
    );
    expect(groups.map((g) => g.round)).toEqual([1, 0, 1]);
  });

  it("adds an empty current-round group on top only when the newest entry is elsewhere", () => {
    const behind = groupByRound([entry(1, 0)], 3);
    expect(behind.map((g) => [g.round, g.entries.length])).toEqual([
      [3, 0],
      [0, 1],
    ]);
    const inside = groupByRound([entry(2, 3), entry(1, 0)], 3);
    expect(inside.map((g) => [g.round, g.entries.length])).toEqual([
      [3, 1],
      [0, 1],
    ]);
    expect(groupByRound([], 2).map((g) => [g.round, g.entries.length])).toEqual(
      [[2, 0]],
    );
  });

  it("groups entries sharing a transactionId and keeps a transaction in the round of its newest entry", () => {
    // Next Round: initiative stamped 3 (newest), advance stamped 2, both in transaction 10.
    const init = { ...entry(11, 3), transactionId: 10 };
    const advance = { ...entry(10, 2), transactionId: 10 };
    const groups = groupByRound([init, advance, entry(9, 2)], 3);
    expect(groups.map((g) => g.round)).toEqual([3, 2]);
    expect(groups[0].transactions.map((t) => t.map((e) => e.id))).toEqual([
      [11, 10],
    ]);
    expect(groups[0].entries.map((e) => e.id)).toEqual([11, 10]);
    expect(groups[1].transactions.map((t) => t.map((e) => e.id))).toEqual([
      [9],
    ]);
  });

  it("is undone only when every entry of the transaction is voided", () => {
    const a = { ...entry(2, 1), transactionId: 1, status: "voided" as const };
    const b = { ...entry(1, 1), transactionId: 1 };
    expect(isUndone([a, b])).toBe(false);
    expect(isUndone([a, { ...b, status: "voided" }])).toBe(true);
    expect(isUndone([])).toBe(false);
  });
});
