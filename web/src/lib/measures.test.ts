import { describe, expect, it } from "vitest";
import {
  feetInches,
  formatHeight,
  formatWeight,
  heightField,
  parseHeight,
  parseWeight,
  unitsFor,
  weightField,
} from "./measures";

describe("units follow the language", () => {
  it("shows en in feet-inches and pounds, fr and the others in metric", () => {
    expect(unitsFor("en")).toBe("imperial");
    expect(unitsFor("en-GB")).toBe("imperial");
    expect(unitsFor("fr")).toBe("metric");
    expect(unitsFor("fr-CA")).toBe("metric");
    expect(unitsFor("de")).toBe("metric");
  });
});

// The same stored value, both languages: 177.8 cm / 72.6 kg is 5'10" / 160 lb.
describe("the same stored value in fr and en", () => {
  it("shows centimetres and kilograms in fr", () => {
    expect(formatHeight(177.8, "metric", "fr")).toMatch(/^177,8\s?cm$/);
    expect(formatWeight(72.6, "metric", "fr")).toMatch(/^72,6\s?kg$/);
  });
  it("shows feet-inches and pounds in en", () => {
    expect(formatHeight(177.8, "imperial", "en")).toBe(`5'10"`);
    expect(formatWeight(72.6, "imperial", "en")).toBe("160 lb");
  });
  it("rounds to the nearest inch and splits feet", () => {
    expect(feetInches(152.4)).toEqual({ feet: 5, inches: 0 });
    expect(feetInches(182.9)).toEqual({ feet: 6, inches: 0 });
    expect(feetInches(81.2)).toEqual({ feet: 2, inches: 8 });
  });
  it("fills a field without the unit, which is in its label", () => {
    expect(heightField(177.8, "metric", "fr")).toBe("177,8");
    expect(heightField(177.8, "imperial", "en")).toBe(`5'10"`);
    expect(weightField(72.6, "metric", "fr")).toBe("72,6");
    expect(weightField(72.6, "imperial", "en")).toBe("160");
    expect(heightField(null, "metric", "fr")).toBe("");
    expect(weightField(null, "imperial", "en")).toBe("");
  });
});

describe("typing a height", () => {
  it("reads centimetres, with a comma or a dot, in metric", () => {
    expect(parseHeight("177,8", "metric")).toEqual({
      value: 177.8,
      invalid: false,
    });
    expect(parseHeight(" 180 cm ", "metric")).toEqual({
      value: 180,
      invalid: false,
    });
  });
  it("reads feet and inches in imperial, a lone number being inches", () => {
    for (const s of [`5'10"`, "5'10", "5 10", "5 ft 10 in", "5ft10"]) {
      expect(parseHeight(s, "imperial"), s).toEqual({
        value: 177.8,
        invalid: false,
      });
    }
    expect(parseHeight("5'", "imperial").value).toBe(152.4);
    expect(parseHeight("70", "imperial").value).toBe(177.8);
  });
  it("clears on empty and flags what is no figure", () => {
    expect(parseHeight("  ", "metric")).toEqual({
      value: null,
      invalid: false,
    });
    expect(parseHeight("tall", "metric").invalid).toBe(true);
    expect(parseHeight("5'x", "imperial").invalid).toBe(true);
    expect(parseHeight("-3", "metric").invalid).toBe(true);
  });
  it("round-trips a stored value through the field of each language", () => {
    for (const cm of [81.2, 152.4, 177.8, 203.2]) {
      expect(
        parseHeight(heightField(cm, "imperial", "en"), "imperial").value,
      ).toBeCloseTo(cm, 0);
      expect(parseHeight(heightField(cm, "metric", "fr"), "metric").value).toBe(
        cm,
      );
    }
  });
});

describe("typing a weight", () => {
  it("reads kilograms in metric and pounds in imperial, stored in kilograms", () => {
    expect(parseWeight("72,6", "metric")).toEqual({
      value: 72.6,
      invalid: false,
    });
    expect(parseWeight("160", "imperial")).toEqual({
      value: 72.6,
      invalid: false,
    });
    expect(parseWeight("160 lb", "imperial").value).toBe(72.6);
  });
  it("clears on empty and flags what is no figure", () => {
    expect(parseWeight("", "metric")).toEqual({ value: null, invalid: false });
    expect(parseWeight("heavy", "imperial").invalid).toBe(true);
  });
  it("round-trips a whole number of pounds", () => {
    for (const lb of [48, 52, 100, 140, 200]) {
      const kg = parseWeight(String(lb), "imperial").value;
      expect(weightField(kg, "imperial", "en")).toBe(String(lb));
    }
  });
});
