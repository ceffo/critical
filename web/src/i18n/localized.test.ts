import { describe, expect, it } from "vitest";
import { editLang, localized } from "./localized";

describe("localized", () => {
  const name = { en: "Magic Missile", fr: "Projectile magique" };

  it("picks the UI language, with or without a region", () => {
    expect(localized("fr", name)).toBe("Projectile magique");
    expect(localized("fr-CA", name)).toBe("Projectile magique");
    expect(localized("en", name)).toBe("Magic Missile");
  });

  it("falls back to English when the language has no body", () => {
    expect(localized("fr", { en: "Armor" })).toBe("Armor");
    expect(localized("de", name)).toBe("Magic Missile");
  });

  it("is empty for a text in no language", () => {
    expect(localized("fr", {})).toBe("");
    expect(localized("fr", undefined)).toBe("");
    expect(localized("fr", { fr: "" })).toBe("");
  });

  // ADR 0023 step 12: a name the player typed in another language only is still shown.
  it("falls back to the first language when it has neither the UI language nor English", () => {
    expect(localized("en", { fr: "Corde" })).toBe("Corde");
    expect(localized("en", { fr: "Corde", de: "Seil" })).toBe("Seil");
  });
});

describe("editLang", () => {
  it("is the UI language without its region (ADR 0023 step 11)", () => {
    expect(editLang("fr-CA")).toBe("fr");
    expect(editLang("en")).toBe("en");
  });
});
