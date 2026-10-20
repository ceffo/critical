import { describe, expect, it } from "vitest";
import {
  alignmentLimit,
  allRaces,
  signed,
  blockedReason,
  emptyPlacement,
  isPlaced,
  place,
  placedValues,
  initialPlacement,
  stepOfField,
  xpOfLevel,
  type StepInputs,
} from "./creationFlow";
import type {
  CreationOptions,
  ScoreMethod,
  ScoresRoll,
} from "@/types/character";

const method = (
  code: string,
  assign: ScoreMethod["assign"],
  base = 0,
  max = 0,
): ScoreMethod => ({
  code,
  rolls: 6,
  dice: 3,
  dropLowest: false,
  assign,
  keep: 6,
  base,
  max,
});

const roll = (
  m: ScoreMethod,
  totals: number[],
  abilities: string[] = [],
  adjustments: Record<string, number> | null = null,
): ScoresRoll => ({
  method: m,
  rolls: totals.map((total, i) => ({
    dice: [total],
    dropped: null,
    total,
    ability: abilities[i] ?? "",
  })),
  adjustments,
  scores: null,
});

describe("stepOfField", () => {
  it("opens the step a refusal belongs to", () => {
    expect(stepOfField("class")).toBe("class");
    expect(stepOfField("xp")).toBe("class");
    expect(stepOfField("raceCode")).toBe("race");
    expect(stepOfField("race")).toBe("race");
    expect(stepOfField("cha")).toBe("scores");
    expect(stepOfField("hpMax")).toBe("extras");
    expect(stepOfField("strExceptional")).toBe("extras");
    expect(stepOfField("skillPoints.climb_walls")).toBe("extras");
    expect(stepOfField("nonweapon")).toBe("proficiencies");
    expect(stepOfField("nonweapon.tumbling")).toBe("proficiencies");
    expect(stepOfField("name")).toBe("identity");
    expect(stepOfField("details.alignment")).toBe("identity");
    expect(stepOfField("details.height")).toBe("identity");
    expect(stepOfField("whatever")).toBe("review");
  });
});

describe("xpOfLevel", () => {
  const options = {
    classes: [
      {
        class: "bard",
        levelXp: [
          { level: 1, xp: 0 },
          { level: 5, xp: 10000 },
        ],
      },
      { class: "mage" },
    ],
  } as unknown as CreationOptions;
  it("reads the server's table, the class's lowest XP for the level", () => {
    expect(xpOfLevel(options, "bard", 5)).toBe(10000);
    expect(xpOfLevel(options, "bard", 1)).toBe(0);
  });
  it("is undefined without the table or the class", () => {
    expect(xpOfLevel(options, "mage", 5)).toBeUndefined();
    expect(xpOfLevel(options, "thief", 5)).toBeUndefined();
    expect(xpOfLevel(null, "bard", 5)).toBeUndefined();
  });
});

describe("alignmentLimit", () => {
  const all = ["a", "b", "c", "d", "e"];
  it("words the server's list: all, all but a few, or the admitted ones", () => {
    expect(alignmentLimit(all, all)).toEqual({ kind: "all" });
    expect(alignmentLimit(["b", "c", "d", "e"], all)).toEqual({
      kind: "except",
      codes: ["a"],
    });
    expect(alignmentLimit(["e", "b"], all)).toEqual({
      kind: "only",
      codes: ["b", "e"],
    });
  });
});

describe("place (the player's arrangement, ADR 0032 step 6)", () => {
  it("method II keeps a total only on its own ability, replacing the other", () => {
    const r = roll(method("II", "keep_per_ability"), [9, 14], ["str", "str"]);
    const p1 = place(r, emptyPlacement(), 0, "str")!;
    expect(p1.str).toEqual([0]);
    expect(place(r, p1, 1, "dex")).toBeNull();
    expect(place(r, p1, 1, "str")!.str).toEqual([1]);
  });

  it("methods III to V use each total once, moving it", () => {
    const r = roll(method("V", "arrange"), [15, 13, 16, 10, 12, 13]);
    let p = place(r, emptyPlacement(), 2, "cha")!;
    p = place(r, p, 2, "dex")!;
    expect(p.cha).toEqual([]);
    expect(p.dex).toEqual([2]);
    p = place(r, p, 0, "dex")!;
    expect(p.dex).toEqual([0]);
    expect(isPlaced(p, 2)).toBe(false);
  });

  it("method VI adds dice to an ability and takes one back (no sum: the server's)", () => {
    const r = roll(method("VI", "add_to_base", 8, 18), [6, 4, 5, 1, 2, 3, 6]);
    let p = place(r, emptyPlacement(), 0, "str")!;
    p = place(r, p, 1, "str")!;
    p = place(r, p, 3, "str")!;
    expect(p.str).toEqual([0, 1, 3]);
    expect(placedValues(r, p).str).toEqual([6, 4, 1]);
    p = place(r, p, 1, "str")!;
    expect(p.str).toEqual([0, 3]);
    p = place(r, p, 3, "dex")!;
    expect(placedValues(r, p)).toEqual({
      str: [6],
      dex: [1],
      con: [],
      int: [],
      wis: [],
      cha: [],
    });
  });

  it("method I places nothing; its totals start on their abilities", () => {
    const r = roll(
      method("I", "in_order"),
      [10, 11, 12, 13, 14, 15],
      ["str", "dex", "con", "int", "wis", "cha"],
    );
    expect(place(r, emptyPlacement(), 0, "str")).toBeNull();
    const p = initialPlacement(r);
    expect(p.wis).toEqual([4]);
    expect(placedValues(r, p).cha).toEqual([15]);
  });

  it("the other methods start with nothing placed", () => {
    const r = roll(method("III", "arrange"), [15, 9]);
    expect(initialPlacement(r)).toEqual(emptyPlacement());
  });
});

describe("blockedReason", () => {
  const ok: StepInputs = {
    cls: "bard",
    race: "half_elf",
    xp: "10000",
    scores: {
      str: "10",
      dex: "15",
      con: "13",
      int: "13",
      wis: "12",
      cha: "16",
    },
    hpMax: "20",
    name: "Lyra",
    alignment: "neutral_good",
    sex: "female",
    alignments: ["neutral_good", "true_neutral"],
  };
  it("blocks only on missing or unreadable input", () => {
    expect(blockedReason("class", ok)).toBeNull();
    expect(blockedReason("class", { ...ok, cls: "" })).toBe("class");
    expect(blockedReason("class", { ...ok, xp: "" })).toBe("xp");
    expect(blockedReason("race", { ...ok, race: "" })).toBe("race");
    expect(
      blockedReason("scores", { ...ok, scores: { ...ok.scores, wis: "x" } }),
    ).toBe("scores");
    expect(blockedReason("extras", { ...ok, hpMax: " " })).toBe("hpMax");
    expect(blockedReason("identity", ok)).toBeNull();
    expect(blockedReason("identity", { ...ok, name: "  " })).toBe("name");
    expect(blockedReason("identity", { ...ok, alignment: "" })).toBe(
      "alignment",
    );
    expect(blockedReason("identity", { ...ok, sex: "" })).toBe("sex");
    expect(blockedReason("review", { ...ok, name: "" })).toBeNull();
  });
  it("blocks the identity on an alignment the class's list leaves out", () => {
    expect(blockedReason("identity", { ...ok, alignment: "lawful_good" })).toBe(
      "alignmentAllowed",
    );
  });
  it("leaves the ranges to the server", () => {
    expect(
      blockedReason("scores", { ...ok, scores: { ...ok.scores, str: "40" } }),
    ).toBeNull();
    expect(blockedReason("extras", { ...ok, hpMax: "0" })).toBeNull();
  });
});

describe("allRaces and signed", () => {
  it("lists the races in the server's Table 8 order, else the classes' union", () => {
    const withTable = {
      classes: [{ class: "bard", races: ["half_elf"] }],
      raceAdjustments: [
        { race: "human", adjustments: {} },
        { race: "elf", adjustments: {} },
      ],
    } as unknown as CreationOptions;
    expect(allRaces(withTable)).toEqual(["human", "elf"]);
    const without = {
      classes: [
        { class: "bard", races: ["human", "half_elf"] },
        { class: "mage", races: ["human", "elf"] },
      ],
    } as unknown as CreationOptions;
    expect(allRaces(without)).toEqual(["human", "half_elf", "elf"]);
  });
  it("signs with a true minus, +0 for none", () => {
    expect(signed(1)).toBe("+1");
    expect(signed(0)).toBe("+0");
    expect(signed(-2)).toBe("−2");
  });
});
