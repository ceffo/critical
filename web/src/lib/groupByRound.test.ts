import { describe, expect, it } from "vitest";
import type { LogEntry } from "@/types/log";
import { groupByRound } from "./groupByRound";

const entry = (id: number, round: number): LogEntry => ({
  id,
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
});
