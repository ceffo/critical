import { describe, expect, it } from "vitest";
import type { Power } from "@/types/powers";
import { EMPTY_DRAFT, asks, refusalOf, requestOf } from "./powerUse";

const power = (over: Partial<Power>): Power => ({
  id: "mind_thrust",
  name: { en: "Mind Thrust" },
  discipline: "telepathy",
  kind: "attack_mode",
  score: { ability: "int", modifier: -4 },
  initial_cost: 7,
  prerequisites: [],
  range: {},
  preparation_time: {},
  area: {},
  description: {},
  ...over,
});

describe("requestOf", () => {
  it("sends no inputs for a power with a fixed cost and its own score", () => {
    expect(requestOf(power({}), EMPTY_DRAFT)).toEqual({ modifiers: [] });
  });

  it("asks for what the power leaves open: variable cost, variable maintenance, a score in the text", () => {
    const p = power({
      score: undefined,
      initial_cost: "variable",
      maintenance: { cost: "variable", period: 1 },
    });
    expect(asks(p)).toEqual({ cost: true, maintenanceCost: true, score: true });
    expect(requestOf(p, EMPTY_DRAFT)).toBeNull();
    expect(
      requestOf(p, {
        ...EMPTY_DRAFT,
        cost: "5",
        maintenanceCost: "2",
        score: "11",
      }),
    ).toEqual({ modifiers: [], cost: 5, maintenanceCost: 2, score: 11 });
    expect(
      requestOf(p, {
        ...EMPTY_DRAFT,
        cost: "0",
        maintenanceCost: "2",
        score: "11",
      }),
    ).toBeNull();
    expect(
      requestOf(p, {
        ...EMPTY_DRAFT,
        cost: "5",
        maintenanceCost: "2",
        score: "26",
      }),
    ).toBeNull();
  });

  it("keeps labelled modifiers within ±20, drops blanks and zeros", () => {
    const d = (modifiers: { value: string; reason: string }[]) => ({
      ...EMPTY_DRAFT,
      modifiers,
    });
    expect(
      requestOf(
        power({}),
        d([
          { value: "-2", reason: " the DM " },
          { value: "", reason: "ignored" },
          { value: "0", reason: "nothing" },
          { value: "+3", reason: "" },
        ]),
      ),
    ).toEqual({
      modifiers: [
        { value: -2, reason: "the DM" },
        { value: 3, reason: "" },
      ],
    });
    expect(requestOf(power({}), d([{ value: "21", reason: "" }]))).toBeNull();
    expect(requestOf(power({}), d([{ value: "x", reason: "" }]))).toBeNull();
  });
});

describe("refusalOf", () => {
  it("refuses a power that needs contact, in the cost or the maintenance", () => {
    expect(refusalOf(power({ initial_cost: "contact" }), EMPTY_DRAFT, 50)).toBe(
      "needs_contact",
    );
    expect(
      refusalOf(
        power({ maintenance: { cost: "contact", period: 1 } }),
        EMPTY_DRAFT,
        50,
      ),
    ).toBe("needs_contact");
  });

  it("refuses when the PSPs left do not cover the full initial cost", () => {
    expect(refusalOf(power({}), EMPTY_DRAFT, 6)).toBe("not_enough_psp");
    expect(refusalOf(power({}), EMPTY_DRAFT, 7)).toBeNull();
    const variable = power({ initial_cost: "variable" });
    expect(refusalOf(variable, { ...EMPTY_DRAFT, cost: "9" }, 8)).toBe(
      "not_enough_psp",
    );
    expect(refusalOf(variable, EMPTY_DRAFT, 8)).toBeNull();
  });
});
