import { describe, expect, it } from "vitest";
import en from "./locales/en/common.json";
import fr from "./locales/fr/common.json";

// The refusal codes of the class entries (the server's, charactersvc create.go) each have wording.
const CODES = [
  "multiclass_not_allowed",
  "multiclass_class_not_allowed",
  "dualclass_not_allowed",
  "prime_requisite_too_low",
  "level_too_low",
  "class_already_left",
  "duplicate_class",
  "too_many_classes",
  "multi_and_dual_class",
  "no_active_class",
];

describe("class refusals", () => {
  for (const [name, c] of [
    ["en", en],
    ["fr", fr],
  ] as const)
    it(`${name} words every code`, () => {
      for (const code of CODES)
        expect(
          (c.creation.classes.errors as Record<string, string>)[code],
          code,
        ).toBeTruthy();
    });
});

// ADR 0050 step 15.1: the four 409 codes of the entry switch (the server's, classentrysvc) have
// wording where the web reads an error code (`apiError.<code>`).
const SWITCH_CODES = [
  "class_switch_not_allowed",
  "class_switch_minimums_not_met",
  "class_switch_alignment_not_allowed",
  "multiclass_class_not_allowed",
];

describe("class entry switch refusals", () => {
  for (const [name, c] of [
    ["en", en],
    ["fr", fr],
  ] as const)
    it(`${name} words every code`, () => {
      for (const code of SWITCH_CODES)
        expect((c.apiError as Record<string, string>)[code], code).toBeTruthy();
    });
});
