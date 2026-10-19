import { describe, expect, it } from "vitest";
import {
  ALIGNMENT_CODES,
  codeOrEmpty,
  hasAlignmentWarning,
  languageKey,
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
  languages: ["Elfique", "Commun"],
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
    const { body } = toBody({ ...toDraft(details), age: " ", languages: "  " });
    expect(body.age).toBeNull();
    expect(body.languages).toEqual([]);
  });
  it("flags an age that is not a whole number", () => {
    expect(toBody({ ...toDraft(details), age: "12a" }).ageInvalid).toBe(true);
    expect(toBody({ ...toDraft(details), age: "-3" }).ageInvalid).toBe(true);
    expect(toBody({ ...toDraft(details), age: "0" }).ageInvalid).toBe(true);
  });
  it("keeps blank entries so the server can reject them", () => {
    expect(
      toBody({ ...toDraft(details), languages: "Commun, ,Nain" }).body
        .languages,
    ).toEqual(["Commun", "", "Nain"]);
  });
});

describe("languageKey", () => {
  it("drops case and accents", () => {
    expect(languageKey("Élfique")).toBe("elfique");
    expect(languageKey("Haut Elfique")).toBe("hautelfique");
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
