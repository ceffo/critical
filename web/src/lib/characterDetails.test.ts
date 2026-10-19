import { describe, expect, it } from "vitest";
import {
  rangeWarnings,
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
  height_cm: 177.8,
  weight_kg: 72.6,
  hair: "",
  eyes: "",
  languages: ["common", "elvish"],
};

describe("an unrelated edit does not drift the measures", () => {
  it("keeps the stored numbers of fields left as toDraft showed them, in en", () => {
    const d = { ...details, height_cm: 180, weight_kg: 75 };
    const draft = { ...toDraft(d, "en"), hair: "red" };
    expect(toBody(draft, "en", d).body).toMatchObject({
      height_cm: 180,
      weight_kg: 75,
    });
    // Without the original the whole-inch field would be sent back: 180.3 and 74.8.
    expect(toBody(draft, "en").body).toMatchObject({
      height_cm: 180.3,
      weight_kg: 74.8,
    });
    // A field the player did change is parsed.
    const typed = { ...draft, weight: "160" };
    expect(toBody(typed, "en", d).body).toMatchObject({
      height_cm: 180,
      weight_kg: 72.6,
    });
  });
});

describe("character details draft", () => {
  it("round-trips the details", () => {
    expect(toBody(toDraft(details))).toEqual({
      ageInvalid: false,
      invalid: [],
      body: details,
    });
  });
  it("shows the same height and weight per language and sends the same numbers", () => {
    expect(toDraft(details, "fr")).toMatchObject({
      height: "177,8",
      weight: "72,6",
    });
    expect(toDraft(details, "en")).toMatchObject({
      height: `5'10"`,
      weight: "160",
    });
    expect(toBody(toDraft(details, "en"), "en").body).toMatchObject({
      height_cm: 177.8,
      weight_kg: 72.6,
    });
    expect(toBody(toDraft(details, "fr"), "fr").body).toMatchObject({
      height_cm: 177.8,
      weight_kg: 72.6,
    });
  });
  it("sends null for an empty height or weight and flags text that is no figure", () => {
    const d = { ...toDraft(details), height: " ", weight: "heavy" };
    const { body, invalid } = toBody(d);
    expect(body.height_cm).toBeNull();
    expect(invalid).toEqual(["weight"]);
    expect(toBody({ ...d, height: "tall" }).invalid).toEqual([
      "height",
      "weight",
    ]);
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

describe("rangeWarnings", () => {
  it("reads each outside-range warning with its value and range, and none without warnings", () => {
    expect(
      rangeWarnings({
        warnings: [
          { code: "alignment_not_allowed", field: "alignment" },
          {
            code: "height_outside_range",
            field: "height",
            value: 210,
            min: 152.4,
            max: 177.8,
          },
          {
            code: "age_outside_range",
            field: "age",
            value: 400,
            min: 16,
            max: 185,
          },
          { code: "weight_outside_range", field: "weight" },
        ],
      }),
    ).toEqual({
      height: { field: "height", value: 210, min: 152.4, max: 177.8 },
      age: { field: "age", value: 400, min: 16, max: 185 },
    });
    expect(rangeWarnings({})).toEqual({});
  });
});
