import { describe, expect, it } from "vitest";
import {
  ALIGNMENT_CODES,
  codeOrEmpty,
  hasAlignmentWarning,
  languageWarning,
  toBody,
  toDraft,
} from "./characterDetails";

const details = {
  alignment: "neutral_good",
  age: 112,
  sex: "male",
  height: "",
  weight: "",
  hair: "",
  eyes: "",
  languages: ["common", "elvish"],
};

describe("character details draft", () => {
  it("round-trips the details", () => {
    expect(toBody(toDraft(details))).toEqual({
      ageInvalid: false,
      body: details,
    });
  });
  it("leaves out an alignment or sex not picked yet", () => {
    const { body } = toBody({ ...toDraft(details), alignment: "", sex: "" });
    expect(body).not.toHaveProperty("alignment");
    expect(body).not.toHaveProperty("sex");
  });
  it("an empty age clears it and no languages is an empty list", () => {
    const { body } = toBody({ ...toDraft(details), age: " ", languages: [] });
    expect(body.age).toBeNull();
    expect(body.languages).toEqual([]);
  });
  it("flags an age that is not a whole number", () => {
    expect(toBody({ ...toDraft(details), age: "12a" }).ageInvalid).toBe(true);
    expect(toBody({ ...toDraft(details), age: "-3" }).ageInvalid).toBe(true);
    expect(toBody({ ...toDraft(details), age: "0" }).ageInvalid).toBe(true);
  });
  it("sends the picked language codes as they are, the server checks them", () => {
    expect(
      toBody({ ...toDraft(details), languages: ["gnoll", "nope"] }).body
        .languages,
    ).toEqual(["gnoll", "nope"]);
  });
  it("copies the character's languages into the draft", () => {
    const draft = toDraft(details);
    expect(draft.languages).toEqual(["common", "elvish"]);
    expect(draft.languages).not.toBe(details.languages);
  });
});

describe("languageWarning", () => {
  it("reads the figures of the languages warning, and an answer without warnings as none", () => {
    expect(
      languageWarning({
        warnings: [
          { code: "alignment_not_allowed" },
          { code: "languages_over_limit", count: 5, limit: 0 },
        ],
      }),
    ).toEqual({ count: 5, limit: 0 });
    expect(languageWarning({ warnings: [] })).toBeUndefined();
    expect(languageWarning({})).toBeUndefined();
  });
});

describe("hasAlignmentWarning", () => {
  it("reads the warning, and an answer without `warnings` (the previous release) as none", () => {
    expect(
      hasAlignmentWarning({
        warnings: [{ code: "alignment_not_allowed", field: "alignment" }],
      }),
    ).toBe(true);
    expect(hasAlignmentWarning({ warnings: [] })).toBe(false);
    expect(hasAlignmentWarning({})).toBe(false);
  });
});

describe("an old release's free text", () => {
  it("is not kept in the draft, so an unrelated save never sends it", () => {
    const draft = toDraft({
      ...details,
      alignment: "Chaotic Good",
      sex: "Homme",
    });
    expect(draft.alignment).toBe("");
    expect(draft.sex).toBe("");
    const { body } = toBody(draft);
    expect(body).not.toHaveProperty("alignment");
    expect(body).not.toHaveProperty("sex");
    expect(codeOrEmpty("lawful_good", ALIGNMENT_CODES)).toBe("lawful_good");
  });
});
