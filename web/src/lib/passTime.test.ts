import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { formatTurnsLeft } from "./passTime";

async function makeT(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t;
}

describe("formatTurnsLeft", () => {
  it.each([
    [150, "150 rounds · 2 h 30", "150 tours · 2 h 30"],
    [59, "59 rounds", "59 tours"],
    [60, "60 rounds · 1 h", "60 tours · 1 h"],
    [61, "61 rounds · 1 h 01", "61 tours · 1 h 01"],
    [1, "1 round", "1 tour"],
  ])("%d turns", async (turns, en, fr) => {
    expect(formatTurnsLeft(await makeT("en"), turns)).toBe(en);
    expect(formatTurnsLeft(await makeT("fr"), turns)).toBe(fr);
  });
});
