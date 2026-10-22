import { describe, expect, it } from "vitest";
import en from "./locales/en/common.json";
import fr from "./locales/fr/common.json";

// The refusal codes of the class entries (the server's, charactersvc create.go) each have wording.
const CODES = [
  "multiclass_not_allowed",
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
