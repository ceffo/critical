import { describe, expect, it } from "vitest";
import { addDie, dieCount, dieName, removeDie, stepConstant } from "./diceFormula";

describe("diceFormula (text edits only)", () => {
  it("names the dice", () => {
    expect(dieName(6)).toBe("d6");
    expect(dieName(100)).toBe("d%");
  });

  it("adds dice, counting up the existing term", () => {
    expect(addDie("", 6)).toBe("d6");
    expect(addDie("d6", 6)).toBe("2d6");
    expect(addDie("2d6", 6)).toBe("3d6");
    expect(addDie("2d6", 4)).toBe("2d6+d4");
    expect(addDie("2d6+3", 4)).toBe("2d6+d4+3");
    expect(addDie("d6 + 1", 6)).toBe("2d6+1");
    expect(addDie("d%", 100)).toBe("2d%");
    expect(addDie("d100", 100)).toBe("2d%");
  });

  it("leaves a subtracted die alone and adds a positive one", () => {
    expect(addDie("-d6", 6)).toBe("-d6+d6");
  });

  it("appends to text it cannot split", () => {
    expect(addDie("2d6*3", 4)).toBe("2d6*3+d4");
    expect(addDie("(", 4)).toBe("(+d4");
  });

  it("removes dice, dropping the term at zero", () => {
    expect(removeDie("3d6", 6)).toBe("2d6");
    expect(removeDie("2d6", 6)).toBe("d6");
    expect(removeDie("d6+2", 6)).toBe("2");
    expect(removeDie("2d%+d4", 100)).toBe("d%+d4");
    expect(removeDie("d4", 6)).toBe("d4");
    expect(removeDie("", 6)).toBe("");
  });

  it("counts the dice of a size for the badge", () => {
    expect(dieCount("2d6+d6+d4+1", 6)).toBe(3);
    expect(dieCount("2d6-d4", 4)).toBe(0);
    expect(dieCount("d%+d100", 100)).toBe(2);
    expect(dieCount("", 6)).toBe(0);
  });

  it("steps the trailing constant", () => {
    expect(stepConstant("d20", 1)).toBe("d20+1");
    expect(stepConstant("d20", -1)).toBe("d20-1");
    expect(stepConstant("d20+1", 1)).toBe("d20+2");
    expect(stepConstant("d20+1", -1)).toBe("d20");
    expect(stepConstant("d20-1", -1)).toBe("d20-2");
    expect(stepConstant("d20-1", 1)).toBe("d20");
    expect(stepConstant("2d6+d4", 1)).toBe("2d6+d4+1");
    expect(stepConstant("", 1)).toBe("1");
  });
});
