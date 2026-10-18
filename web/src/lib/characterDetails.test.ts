import { describe, expect, it } from "vitest";
import { languageKey, toBody, toDraft } from "./characterDetails";

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
