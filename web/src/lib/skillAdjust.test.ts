import { describe, expect, it } from "vitest";
import {
  checkAdjustedTarget,
  maxSkillAdjustment,
  previewTarget,
  stepSkillAdjustment,
} from "./skillAdjust";

describe("stepSkillAdjustment", () => {
  it.each([
    ["proficiency", 0, 1, 1],
    ["proficiency", 0, -1, -1],
    ["proficiency", 20, 1, 20],
    ["proficiency", -20, -1, -20],
    ["bardSkill", 0, 1, 5],
    ["bardSkill", 0, -1, -5],
    ["bardSkill", 98, 1, 100],
    ["bardSkill", -100, -1, -100],
  ] as const)("%s %i %+i → %i", (kind, value, dir, want) => {
    expect(stepSkillAdjustment(kind, value, dir)).toBe(want);
  });

  it("limits follow the server's ranges", () => {
    expect(maxSkillAdjustment("proficiency")).toBe(20);
    expect(maxSkillAdjustment("bardSkill")).toBe(100);
  });
});

describe("targets", () => {
  it("previews the target with the pending adjustment", () => {
    expect(previewTarget(16, -2)).toBe(14);
    expect(previewTarget(85, 10)).toBe(95);
  });

  it("reads the adjusted target, or derives it on an entry logged before adjustments", () => {
    expect(
      checkAdjustedTarget({ target: 16, adjustment: -2, adjustedTarget: 14 }),
    ).toBe(14);
    expect(checkAdjustedTarget({ target: 16 })).toBe(16);
  });
});
