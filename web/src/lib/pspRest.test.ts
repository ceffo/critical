import { describe, expect, it } from "vitest";
import type { BuffEffect } from "@/types/buffs";
import {
  defaultRow,
  freeRows,
  payingRows,
  previewRest,
  visibleRecovery,
} from "./pspRest";

const rows = [
  { activity: "a", perHour: 0 },
  { activity: "b", perHour: 6 },
  { activity: "c", perHour: 12 },
  { activity: "d", perHour: 12, requiresProficiency: "gate" },
];

const buff = (over: Partial<BuffEffect>): BuffEffect => ({
  id: "1",
  name: "x",
  charges: 0,
  duration: 0,
  effects: {},
  note: "",
  startRound: 0,
  chargesTotal: 0,
  ...over,
});

describe("the rest popover's rows (ADR 0047 items 5 and 11)", () => {
  it("hides a gated row without its proficiency", () => {
    expect(visibleRecovery(rows, new Set()).map((r) => r.activity)).toEqual([
      "a",
      "b",
      "c",
    ]);
  });

  it("shows the gated row once the proficiency is held, keeping the table's order", () => {
    expect(
      visibleRecovery(rows, new Set(["gate"])).map((r) => r.activity),
    ).toEqual(["a", "b", "c", "d"]);
  });

  it("a pool with no table shows no row", () => {
    expect(visibleRecovery(undefined, new Set())).toEqual([]);
  });

  it("opens on the fastest row, the first of equals", () => {
    expect(defaultRow(rows.slice(0, 3))?.activity).toBe("c");
    expect(defaultRow(visibleRecovery(rows, new Set(["gate"])))?.activity).toBe(
      "c",
    );
    expect(defaultRow([])).toBeUndefined();
  });
});

describe("the preview matches the server's figure", () => {
  it("adds hours times the rate", () => {
    expect(previewRest({ current: 71, max: 109 }, 6, 3)).toEqual({
      gain: 18,
      after: 89,
    });
  });

  it("caps the result at the maximum and counts only what lands", () => {
    expect(previewRest({ current: 100, max: 109 }, 12, 8)).toEqual({
      gain: 9,
      after: 109,
    });
  });

  it("rounds a fraction of a PSP down, as the ledger does", () => {
    expect(previewRest({ current: 10, max: 100 }, 3, 1.5)).toEqual({
      gain: 4,
      after: 14,
    });
  });

  it("a rate of 0 or no hours changes nothing", () => {
    expect(previewRest({ current: 10, max: 100 }, 0, 4).after).toBe(10);
    expect(previewRest({ current: 10, max: 100 }, 6, 0).after).toBe(10);
  });
});

describe("the rows a rest stops (ADR 0047 item 4)", () => {
  const list = [
    buff({ id: "1", powerId: "p", maintenanceCost: 8, maintenancePeriod: 1 }),
    buff({ id: "2", powerId: "q", maintenanceCost: 0 }),
    buff({ id: "3", target: "Brann", maintenanceCost: 0 }),
    buff({ id: "4", target: "Brann", maintenanceCost: 2 }),
    buff({ id: "5", name: "Bless", spellId: "bless" }),
  ];

  it("a row that costs PSPs stops, a spell's buff is never touched", () => {
    expect(payingRows(list).map((b) => b.id)).toEqual(["1", "4"]);
  });

  it("rows that cost nothing stay up", () => {
    expect(freeRows(list).map((b) => b.id)).toEqual(["2", "3"]);
  });
});
