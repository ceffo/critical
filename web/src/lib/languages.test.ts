import { describe, expect, it } from "vitest";
import { raceLanguages } from "./languages";

const options = {
  raceLanguages: [
    { race: "dwarf", native: "dwarvish", languages: ["common", "dwarvish"] },
  ],
};

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
