import { describe, expect, it } from "vitest";
import type { HpRoll } from "@/types/character";
import { EMPTY_DRAFT, mergeDraft, settle, type FlowDraft } from "./flow";

// A level 1 fighter with CON 15 rolled 9 hit points.
const roll: HpRoll = {
  class: "fighter",
  level: 1,
  levels: [
    {
      level: 1,
      hitDie: 10,
      rolled: 8,
      counted: 8,
      conAdjustment: 1,
      gain: 9,
      fixed: false,
    },
  ],
  total: 9,
};
const rolled: FlowDraft = {
  ...EMPTY_DRAFT,
  cls: "fighter",
  scores: { ...EMPTY_DRAFT.scores, con: "15" },
  hpRoll: roll,
  hpRollFor: { cls: "fighter", con: 15, xp: 0 },
  hpMax: "9",
};

describe("settle (an HP roll stays current only for its class, Constitution and XP, ADR 0032 step 7)", () => {
  it("the same inputs keep the roll", () => {
    expect(settle({ ...rolled, details: { ...rolled.details } }, "0")).toEqual(
      rolled,
    );
  });

  it.each([
    ["the level (its XP)", rolled, "2000"],
    ["the class", { ...rolled, cls: "paladin" }, "0"],
    [
      "the Constitution",
      { ...rolled, scores: { ...rolled.scores, con: "16" } },
      "0",
    ],
  ])("a change of %s drops the roll and its total, and says so", (_, d, xp) => {
    const s = settle(d, xp);
    expect(s.hpRoll).toBeNull();
    expect(s.hpRollFor).toBeNull();
    expect(s.hpMax).toBe("");
    expect(s.hpOutdated).toBe(true);
  });

  it("a total the player typed over the roll is kept", () => {
    const s = settle({ ...rolled, hpMax: "12", cls: "paladin" }, "0");
    expect(s.hpRoll).toBeNull();
    expect(s.hpMax).toBe("12");
  });

  it("typed HP without a roll is left alone", () => {
    const typed = { ...EMPTY_DRAFT, hpMax: "30" };
    expect(settle(typed, "5000")).toBe(typed);
  });
});

describe("mergeDraft (a late reply never rebuilds the details from an older copy)", () => {
  it("keeps the details typed since a roll left, and both rolls of a pair", () => {
    let d: FlowDraft = { ...EMPTY_DRAFT };
    d = mergeDraft(d, { details: { height: "180" } });
    d = mergeDraft(d, { details: { hair: "red" } }); // typed while the weight roll is pending
    d = mergeDraft(d, { details: { weight: "75" } }); // the weight reply, built from no copy
    expect(d.details).toMatchObject({
      height: "180",
      hair: "red",
      weight: "75",
    });
  });
});
