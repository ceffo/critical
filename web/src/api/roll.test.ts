import { describe, expect, it } from "vitest";
import type { SaveBreakdown } from "@/types/combat";
import { normalizeSaveAnswer } from "./roll";

const base = {
  d20Roll: 12,
  modifiers: [],
  adjustment: 0,
  rollTotal: 12,
  adjustedTarget: 14,
  logEntryId: 7,
};

describe("normalizeSaveAnswer (transition: old and new server shapes)", () => {
  it("keeps the new shape as is", () => {
    const raw = {
      ...base,
      category: "Spells",
      target: 14,
      passed: false,
      margin: -2,
    } as SaveBreakdown;
    expect(normalizeSaveAnswer(raw)).toEqual(raw);
  });

  it("reads the previous release's names", () => {
    const raw = {
      ...base,
      saveCategory: "Spells",
      baseThac0: 14,
      hit: false,
      hitMargin: -2,
    } as unknown as SaveBreakdown;
    expect(normalizeSaveAnswer(raw)).toMatchObject({
      category: "Spells",
      target: 14,
      passed: false,
      margin: -2,
      d20Roll: 12,
    });
  });

  it("a passed save with a zero margin stays passed and zero", () => {
    const raw = {
      ...base,
      saveCategory: "Spells",
      baseThac0: 12,
      hit: true,
      hitMargin: 0,
    } as unknown as SaveBreakdown;
    expect(normalizeSaveAnswer(raw)).toMatchObject({
      passed: true,
      margin: 0,
      target: 12,
    });
  });
});
