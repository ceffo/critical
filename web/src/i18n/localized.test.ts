import { describe, expect, it } from "vitest";
import { localized } from "./localized";

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
});
