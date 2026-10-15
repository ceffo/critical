import { describe, expect, it } from "vitest";
import type { LogEntry, LogKind, LogStatus } from "@/types/log";
import { lastDamage, lastHpChange, lastSaves } from "./moiLog";

let nextId = 100;
function entry(kind: LogKind, over: Partial<LogEntry> = {}): LogEntry {
  const id = nextId--;
  return {
    id,
    characterId: 1,
    round: 0,
    kind,
    summary: "",
    hpDelta: 0,
    effectiveHpDelta: 0,
    note: "",
    effectiveSummary: "",
    status: "active" as LogStatus,
    payload: {},
    effectivePayload: {},
    snapshots: [],
    corrections: [],
    createdAt: "2026-10-15T20:00:00Z",
    ...over,
  };
}

const hp = (delta: number, source: string, status: LogStatus = "active") =>
  entry("hp_change", {
    hpDelta: delta,
    effectiveHpDelta: status === "voided" ? 0 : delta,
    summary: source,
    effectiveSummary: source,
    status,
  });

const save = (
  category: string,
  d20: number,
  passed: boolean,
  status: LogStatus = "active",
) =>
  entry("save", {
    payload: { category, d20Roll: d20, passed },
    effectivePayload: { category, d20Roll: d20, passed },
    status,
  });

describe("lastHpChange", () => {
  it("is the newest HP change still in force, skipping voided ones and other kinds", () => {
    const log = [
      save("spell", 12, true),
      hp(-4, "goblin", "voided"),
      hp(3, "potion"),
      hp(-9, "orc axe"),
    ];
    expect(lastHpChange(log)).toEqual({ delta: 3, source: "potion" });
  });

  it("skips a heal capped to nothing (applied 0)", () => {
    const capped = { ...hp(5, "potion at full"), effectiveHpDelta: 0 };
    expect(lastHpChange([capped, hp(-2, "rat")])).toEqual({
      delta: -2,
      source: "rat",
    });
  });

  it("uses the amended source", () => {
    const amended = { ...hp(-6, "orc"), effectiveSummary: "ogre" };
    expect(lastHpChange([amended])?.source).toBe("ogre");
  });

  it("is null on an empty log", () => {
    expect(lastHpChange([])).toBeNull();
  });
});

describe("lastDamage", () => {
  it("is the newest change when it is damage, else 0", () => {
    expect(lastDamage([hp(-9, "orc axe"), hp(4, "potion")])).toBe(9);
    expect(lastDamage([hp(4, "potion"), hp(-9, "orc axe")])).toBe(0);
    expect(lastDamage([])).toBe(0);
  });
});

describe("lastSaves", () => {
  it("keeps the newest non-voided save per category, adjusted payload first", () => {
    const adjusted = {
      ...save("breath", 9, false),
      effectivePayload: {
        category: "breath",
        d20Roll: 9,
        passed: true,
        adjustment: 4,
      },
    };
    const log = [
      save("spell", 3, false, "voided"),
      save("spell", 15, true),
      adjusted,
      save("spell", 2, false),
    ];
    expect(lastSaves(log)).toEqual({
      spell: { d20: 15, passed: true },
      breath: { d20: 9, passed: true },
    });
  });

  it("ignores entries without a usable payload", () => {
    expect(
      lastSaves([entry("save", { payload: null, effectivePayload: null })]),
    ).toEqual({});
  });
});
