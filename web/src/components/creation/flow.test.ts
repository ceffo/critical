import { describe, expect, it } from "vitest";
import type { CreationOptions, HpRoll } from "@/types/character";
import {
  EMPTY_DRAFT,
  buildBody,
  checksRequest,
  mergeDraft,
  settle,
  type FlowDraft,
} from "./flow";

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
    expect("body" in out && out.body.details?.height_cm).toBe(176.5);
  });
  it("reads the figures in the language's units", () => {
    const fr = {
      ...draft,
      details: { ...draft.details, height: "176,5" },
    };
    const out = buildBody(options, fr, "2500", "fr");
    expect("body" in out && out.body.details?.height_cm).toBe(176.5);
    expect("body" in out && out.body.details?.weight_kg).toBe(70.5);
  });
  it("leaves a blank alignment out and carries the entered values", () => {
    const out = buildBody(options, draft, "2500", "en");
    if (!("body" in out)) throw new Error("no body");
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
  it("names the field of an unreadable value", () => {
    expect(
      buildBody(options, { ...draft, hpMax: "six" }, "2500", "fr"),
    ).toEqual({ error: { field: "hpMax", code: "out_of_range" } });
    expect(
      buildBody(
        options,
        { ...draft, details: { ...draft.details, weight: "lourd" } },
        "2500",
        "en",
      ),
    ).toEqual({ error: { field: "details.weight", code: "not_a_number" } });
  });
});

describe("checksRequest (what the class and skills steps ask the server)", () => {
  const options = {
    classes: [{ class: "thief", skills: ["open_locks", "pick_pockets"] }],
  } as unknown as CreationOptions;

  it("sends only the scores and points that read as whole numbers, and the XP", () => {
    const draft: FlowDraft = {
      ...EMPTY_DRAFT,
      cls: "thief",
      scores: { ...EMPTY_DRAFT.scores, dex: "8", str: "x", con: "" },
      skillPoints: { open_locks: "25", pick_pockets: "" },
    };
    expect(checksRequest(options, draft, "1250")).toEqual({
      class: "thief",
      scores: { dex: 8 },
      xp: 1250,
      skillPoints: { open_locks: 25 },
      proficiencies: { nonweapon: [] },
    });
  });

  it("sends no XP while it does not read", () => {
    expect(checksRequest(options, EMPTY_DRAFT, "").xp).toBeNull();
  });
});
