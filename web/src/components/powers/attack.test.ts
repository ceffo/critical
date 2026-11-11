import { describe, expect, it } from "vitest";
import { EMPTY_ATTACK, attackNeeds, attackRequest } from "./attack";

describe("attackRequest", () => {
  it("needs a target", () => {
    expect(attackRequest(EMPTY_ATTACK)).toBeNull();
    expect(attackRequest({ ...EMPTY_ATTACK, target: "Orc" })).toEqual({
      target: "Orc",
      mind: "closed",
      level: 0,
      modifiers: [],
    });
  });

  it("sends the DM's two rolls only with a defence, each 1 to 20", () => {
    const d = { ...EMPTY_ATTACK, target: "Orc", defence: "mind_blank" };
    expect(attackRequest(d)).toBeNull();
    const rolls = [
      { roll: "10", success: true },
      { roll: "3", success: false },
    ];
    expect(attackRequest({ ...d, rolls })).toEqual({
      target: "Orc",
      mind: "closed",
      level: 0,
      defence: "mind_blank",
      rolls: [
        { roll: 10, success: true },
        { roll: 3, success: false },
      ],
      modifiers: [],
    });
    expect(
      attackRequest({ ...d, rolls: [rolls[0], { roll: "21", success: true }] }),
    ).toBeNull();
  });

  it("keeps the labelled modifiers that are not zero or blank", () => {
    const d = {
      ...EMPTY_ATTACK,
      target: "Orc",
      level: "5",
      modifiers: [
        { value: "-2", reason: " medium range " },
        { value: "", reason: "" },
        { value: "0", reason: "x" },
      ],
    };
    expect(attackRequest(d)?.modifiers).toEqual([
      { value: -2, reason: "medium range" },
    ]);
    expect(
      attackRequest({ ...d, modifiers: [{ value: "30", reason: "" }] }),
    ).toBeNull();
  });
});

describe("attackNeeds", () => {
  it("names what the request lacks, and is empty exactly when a request is made", () => {
    expect(attackNeeds(EMPTY_ATTACK)).toEqual(["target"]);
    const named = { ...EMPTY_ATTACK, target: "Orc" };
    expect(attackNeeds(named)).toEqual([]);
    expect(attackNeeds({ ...named, level: "x" })).toEqual(["level"]);
    const defended = { ...named, defence: "mind_blank" };
    expect(attackNeeds(defended)).toEqual(["rolls"]);
    expect(
      attackNeeds({
        ...defended,
        rolls: [
          { roll: "10", success: true },
          { roll: "21", success: false },
        ],
      }),
    ).toEqual(["rolls"]);
    for (const d of [EMPTY_ATTACK, named, defended]) {
      expect(attackRequest(d) === null).toBe(attackNeeds(d).length > 0);
    }
  });
});
