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
  effectiveSummary: `e${id}`,
  status: "active",
  payload: {},
  snapshots: [],
  corrections: [],
  createdAt: "2026-10-17T20:00:00Z",
});

describe("groupByRound", () => {
  it("groups newest round first and keeps entry order", () => {
    const groups = groupByRound([entry(4, 2), entry(3, 2), entry(2, 0), entry(1, 0)], 2);
    expect(groups.map((g) => g.round)).toEqual([2, 0]);
    expect(groups[0].entries.map((e) => e.id)).toEqual([4, 3]);
    expect(groups[1].entries.map((e) => e.id)).toEqual([2, 1]);
  });

  it("always has a (possibly empty) group for the current round", () => {
    const groups = groupByRound([entry(1, 0)], 3);
    expect(groups.map((g) => [g.round, g.entries.length])).toEqual([
      [3, 0],
      [0, 1],
    ]);
  });
});
