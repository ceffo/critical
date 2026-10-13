import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { formatRoundsLeft, previewRounds } from "./passTime";

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

describe("formatRoundsLeft", () => {
  it.each([
    [150, "150 rd · 2 h 30", "150 rd. · 2 h 30"],
    [59, "59 rd", "59 rd."],
    [60, "60 rd · 1 h", "60 rd. · 1 h"],
    [61, "61 rd · 1 h 01", "61 rd. · 1 h 01"],
    [1, "1 rd", "1 rd."],
  ])("%d rounds", async (rounds, en, fr) => {
    expect(formatRoundsLeft(await makeT("en"), rounds)).toBe(en);
    expect(formatRoundsLeft(await makeT("fr"), rounds)).toBe(fr);
  });
});

describe("previewRounds", () => {
  it.each([
    ["1.5", 90],
    ["1,5", 90],
    ["0.02", 1],
    ["0.01", 0],
    ["1.15", 69],
    ["8", 480],
  ])("%s h → %d", (text, want) => expect(previewRounds(text)).toBe(want));

  it.each(["", "abc", "-1", "1.5.2"])("%j is not a number", (text) =>
    expect(previewRounds(text)).toBeNull(),
  );
});
