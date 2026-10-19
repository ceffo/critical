import { describe, expect, it } from "vitest";
import { identityLine, toPickKey } from "./rosterCard";

const word = (key: string, code: string) => `${key}.${code}`;

describe("roster card identity", () => {
  it("joins race and alignment", () => {
    expect(identityLine({ race: "elf", alignment: "true_neutral" }, word)).toBe(
      "details.races.elf · details.alignments.true_neutral",
    );
  });
  it("leaves out an alignment still to choose, with no stray separator", () => {
    expect(identityLine({ race: "elf", alignment: "" }, word)).toBe(
      "details.races.elf",
    );
  });
  it("leaves out a missing race, with no stray separator", () => {
    expect(identityLine({ race: "", alignment: "true_neutral" }, word)).toBe(
      "details.alignments.true_neutral",
    );
    expect(identityLine({ race: "", alignment: "" }, word)).toBe("");
  });
  it("names the tag by what is missing", () => {
    expect(toPickKey([])).toBeNull();
    expect(toPickKey(["sex"])).toBe("roster.toPick.sex");
    expect(toPickKey(["alignment"])).toBe("roster.toPick.alignment");
    expect(toPickKey(["alignment", "sex"])).toBe("roster.toPick.alignment_sex");
  });
});
