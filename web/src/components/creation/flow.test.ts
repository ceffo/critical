import { describe, expect, it } from "vitest";
import type {
  CreationOptions,
  HpRoll,
  ScoreMethod,
  ScoresRoll,
} from "@/types/character";
import {
  EMPTY_DRAFT,
  alignmentLimit,
  allRaces,
  blockedReason,
  buildBody,
  emptyPlacement,
  initialPlacement,
  isPlaced,
  mergeDraft,
  place,
  placedValues,
  refusalError,
  settle,
  signed,
  stepOfField,
  xpOfLevel,
  type FlowDraft,
  type StepInputs,
} from "./flow";

// A level 1 fighter with CON 15 rolled 9 hit points.
const hpRoll: HpRoll = {
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
  hpRoll,
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

  it("clears the picks when the class changes (each was spent at the old class's cost)", () => {
    let d: FlowDraft = { ...EMPTY_DRAFT, cls: "fighter" };
    d = mergeDraft(d, {
      picks: [{ id: "tumbling", slots: 1 }],
      weaponPicks: [{ type: "long_sword", slots: 2 }],
    });
    d = mergeDraft(d, { cls: "fighter" }); // the same class again keeps them
    expect(d.picks).toEqual([{ id: "tumbling", slots: 1 }]);
    d = mergeDraft(d, { level: 2 }); // any other change keeps them
    expect(d.weaponPicks).toEqual([{ type: "long_sword", slots: 2 }]);
    expect(
      buildBody({ classes: [] } as unknown as CreationOptions, d, "0", "fr")
        .body.proficiencies,
    ).toEqual({
      nonweapon: [{ id: "tumbling", slots: 1 }],
      weapon: [{ type: "long_sword", slots: 2 }],
    });
    // A specialization or a weapon type may not be the new class's (ADR 0037 steps 7 and 9).
    d = mergeDraft(d, { cls: "mage" });
    expect(d.picks).toEqual([]);
    expect(d.weaponPicks).toEqual([]);
    expect(
      buildBody({ classes: [] } as unknown as CreationOptions, d, "0", "fr")
        .body.proficiencies,
    ).toBeUndefined();
  });
});

describe("buildBody", () => {
  const options = {
    classes: [
      {
        class: "bard",
        races: ["half_elf"],
        exceptionalStrengthRaces: [],
        skills: ["climb_walls"],
        alignments: ["neutral_good"],
      },
    ],
  } as unknown as CreationOptions;
  const draft: FlowDraft = {
    ...EMPTY_DRAFT,
    cls: "bard",
    race: "half_elf",
    scores: { str: "10", dex: "15", con: "12", int: "14", wis: "9", cha: "16" },
    hpMax: "6",
    name: "Lyra",
    skillPoints: { climb_walls: "15" },
    details: {
      ...EMPTY_DRAFT.details,
      sex: "female",
      // A rolled 176.5 cm shows as 5'9" in English; 5'9" read back is 175.3 cm.
      height: `5'9"`,
      rolledHeight: 176.5,
      weight: "70,5",
    },
  };
  it("sends a rolled height as rolled, not as its rounded display", () => {
    const out = buildBody(options, draft, "2500", "en");
    expect(out.error).toBeNull();
    expect(out.body.details?.height_cm).toBe(176.5);
  });
  it("reads the figures in the language's units", () => {
    const fr = {
      ...draft,
      details: { ...draft.details, height: "176,5" },
    };
    const out = buildBody(options, fr, "2500", "fr");
    expect(out.body.details?.height_cm).toBe(176.5);
    expect(out.body.details?.weight_kg).toBe(70.5);
  });
  it("leaves a blank alignment out and carries the entered values", () => {
    const out = buildBody(options, draft, "2500", "en");
    expect(out.body.details).not.toHaveProperty("alignment");
    expect(out.body).toMatchObject({
      name: "Lyra",
      class: "bard",
      raceCode: "half_elf",
      xp: 2500,
      hpMax: 6,
      skillPoints: { climb_walls: 15 },
    });
    // ADR 0033 step 14: the skills read the worn armour; the body enters none.
    expect(out.body).not.toHaveProperty("skillArmor");
  });
  it("names the field of an unreadable value and leaves it out of the body", () => {
    const six = buildBody(options, { ...draft, hpMax: "six" }, "2500", "fr");
    expect(six.error).toEqual({ field: "hpMax", code: "out_of_range" });
    // The draft is still asked, the value left out: the server lists it missing.
    expect(six.body.hpMax).toBeNull();
    expect(six.body.xp).toBe(2500);
    expect(
      buildBody(
        options,
        { ...draft, details: { ...draft.details, weight: "lourd" } },
        "2500",
        "en",
      ).error,
    ).toEqual({ field: "details.weight", code: "not_a_number" });
    // The first unreadable value is named; the others are left out too.
    const two = buildBody(
      options,
      {
        ...draft,
        scores: { ...draft.scores, str: "x" },
        skillPoints: { climb_walls: "lots" },
      },
      "2500",
      "en",
    );
    expect(two.error).toEqual({ field: "str", code: "out_of_range" });
    expect(two.body.str).toBeNull();
    expect(two.body.skillPoints).toEqual({});
  });

  it("sends each pool's picks only when it has some (ADR 0037 step 14)", () => {
    const none = buildBody(options, draft, "2500", "en");
    expect(none.body.proficiencies).toBeUndefined();
    const weapons = buildBody(
      options,
      { ...draft, weaponPicks: [{ type: "dagger", slots: 1 }] },
      "2500",
      "en",
    );
    expect(weapons.body.proficiencies).toEqual({
      weapon: [{ type: "dagger", slots: 1 }],
    });
  });
});

describe("buildBody on a raw draft (what the class and extras steps ask the server)", () => {
  const options = {
    classes: [
      {
        class: "thief",
        races: ["elf"],
        exceptionalStrengthRaces: [],
        skills: ["open_locks", "pick_pockets"],
      },
    ],
  } as unknown as CreationOptions;

  it("sends the scores and points that read as whole numbers, the XP, and nothing for the blanks", () => {
    const draft: FlowDraft = {
      ...EMPTY_DRAFT,
      cls: "thief",
      scores: { ...EMPTY_DRAFT.scores, dex: "8", con: "" },
      skillPoints: { open_locks: "25", pick_pockets: "" },
      race: "elf",
      details: { ...EMPTY_DRAFT.details, languages: ["elvish", "orcish"] },
    };
    const { body, error } = buildBody(options, draft, "1250", "fr");
    expect(error).toBeNull();
    expect(body).toMatchObject({
      class: "thief",
      raceCode: "elf",
      str: null,
      dex: 8,
      con: null,
      hpMax: null,
      xp: 1250,
      skillPoints: { open_locks: 25 },
    });
    // ADR 0037 step 6: the server counts the languages into the non-weapon pool.
    expect(body.details?.languages).toEqual(["elvish", "orcish"]);
  });

  it("sends no XP while it does not read", () => {
    expect(buildBody(options, EMPTY_DRAFT, "", "fr").body.xp).toBeNull();
  });
});

describe("refusalError", () => {
  it("words an over_budget with the server's figures and keeps any other code", () => {
    expect(refusalError({ field: "str", code: "out_of_range" })).toEqual({
      field: "str",
      code: "out_of_range",
      message: "out_of_range",
    });
    const over = refusalError({
      field: "nonweapon",
      code: "over_budget",
      spent: 8,
      limit: 7,
    });
    expect(over.code).toBe("over_budget");
    expect(over.field).toBe("nonweapon");
    expect(over.message).toBeTruthy();
  });
});

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
  it("opens the step a refusal belongs to, by the server's field names", () => {
    expect(stepOfField("class")).toBe("class");
    expect(stepOfField("xp")).toBe("class");
    expect(stepOfField("race")).toBe("race");
    expect(stepOfField("cha")).toBe("scores");
    expect(stepOfField("hpMax")).toBe("extras");
    expect(stepOfField("strExceptional")).toBe("extras");
    expect(stepOfField("skillPoints.climb_walls")).toBe("extras");
    expect(stepOfField("nonweapon")).toBe("proficiencies");
    expect(stepOfField("nonweapon.tumbling")).toBe("proficiencies");
    expect(stepOfField("weapon.dagger")).toBe("proficiencies");
    // ADR 0037 step 18: the languages are picked on the proficiency step.
    expect(stepOfField("details.languages")).toBe("proficiencies");
    expect(stepOfField("name")).toBe("identity");
    expect(stepOfField("details.alignment")).toBe("identity");
    expect(stepOfField("details.height")).toBe("identity");
    // The web's own names are none of the server's: they stay on the review.
    expect(stepOfField("raceCode")).toBe("review");
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
