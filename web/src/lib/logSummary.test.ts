import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import { entryText } from "./logSummary";

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

function entry(
  kind: LogEntry["kind"],
  payload: unknown,
  summary = "server summary",
): LogEntry {
  return {
    id: 1,
    characterId: 1,
    round: 0,
    kind,
    summary,
    hpDelta: 0,
    effectiveHpDelta: 0,
    effectiveSummary: summary,
    status: "active",
    payload,
    snapshots: [],
    corrections: [],
    createdAt: "2026-10-12T00:00:00Z",
  };
}

describe("entryText", () => {
  it.each([
    [
      { action: "activate", buff: "Stoneskin", charges: 6 },
      "Activated Stoneskin (6 ch)",
    ],
    [
      { action: "activate", buff: "Mirror Image", rounds: 27, charges: 4 },
      "Activated Mirror Image (27 rd, 4 ch)",
    ],
    [
      { action: "activate", buff: "Power Gem" },
      "Activated Power Gem (until dismissed)",
    ],
    [
      { action: "edit", buff: "Strength", fromRounds: 10, rounds: 3 },
      "Strength: 10 rd → 3 rd",
    ],
    [
      { action: "edit", buff: "Strength", fromRounds: 10, ended: true },
      "Strength: 10 rd → ended",
    ],
    [
      { action: "absorb", buff: "Stoneskin", charges: 2 },
      "Stoneskin absorbed a hit (2 left)",
    ],
    [
      { action: "absorb", buff: "Stoneskin", ended: true },
      "Stoneskin absorbed a hit, ended",
    ],
    [{ action: "dismiss", buff: "Haste" }, "Dismissed Haste"],
    [{ action: "next_round", fromRound: 0, toRound: 1 }, "Round 0 → 1"],
    [
      {
        action: "next_round",
        fromRound: 4,
        toRound: 5,
        expired: ["Strength", "Haste"],
      },
      "Round 4 → 5 — expired: Strength, Haste",
    ],
  ])("renders %j", async (payload, want) => {
    const t = await makeT("en");
    const kind =
      payload.action === "next_round" ? "round_advance" : "buff_change";
    expect(entryText(entry(kind, payload), t)).toBe(want);
  });

  it("translates to French", async () => {
    const t = await makeT("fr");
    expect(
      entryText(
        entry("buff_change", {
          action: "absorb",
          buff: "Stoneskin",
          charges: 2,
        }),
        t,
      ),
    ).toBe("Stoneskin a absorbé un coup (reste 2)");
  });

  it("falls back to the server summary", async () => {
    const t = await makeT("en");
    expect(entryText(entry("hp_change", {}, "orc axe"), t)).toBe("orc axe");
    expect(entryText(entry("buff_change", {}, "legacy"), t)).toBe("legacy");
  });
});
