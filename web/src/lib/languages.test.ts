import { describe, expect, it } from "vitest";
import { additionalLanguages, languageLimit, raceLanguages } from "./languages";

const options = {
  languageLimits: [
    { intMin: 1, intMax: 1, limit: 0 },
    { intMin: 2, intMax: 8, limit: 1 },
    { intMin: 14, intMax: 15, limit: 4 },
  ],
  raceLanguages: [
    { race: "dwarf", native: "dwarvish", languages: ["common", "dwarvish"] },
  ],
};

describe("languageLimit", () => {
  it("reads the served band of the score", () => {
    expect(languageLimit(options, 1)).toBe(0);
    expect(languageLimit(options, 8)).toBe(1);
    expect(languageLimit(options, 15)).toBe(4);
  });
  it("is unknown for no score, a score outside the bands, or a server without bands", () => {
    expect(languageLimit(options, null)).toBeUndefined();
    expect(languageLimit(options, Number.NaN)).toBeUndefined();
    expect(languageLimit(options, 10)).toBeUndefined();
    expect(languageLimit({}, 14)).toBeUndefined();
  });
});

describe("raceLanguages", () => {
  it("is the race's own language and list, empty for an unknown race or a previous release", () => {
    expect(raceLanguages(options, "dwarf")).toEqual({
      native: "dwarvish",
      list: ["common", "dwarvish"],
    });
    expect(raceLanguages(options, "elf")).toEqual({ native: "", list: [] });
    expect(raceLanguages({}, "dwarf")).toEqual({ native: "", list: [] });
  });
});

describe("additionalLanguages", () => {
  it("leaves the native language out of the count", () => {
    expect(additionalLanguages(["common", "elvish"], "common")).toEqual([
      "elvish",
    ]);
    expect(additionalLanguages(["common"], "")).toEqual(["common"]);
  });
});
