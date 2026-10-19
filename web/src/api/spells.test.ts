import { describe, expect, it } from "vitest";
import type { CastResult, ExposureResult } from "@/types/spells";
import { normalizeCastAnswer, normalizeExposureAnswer } from "./spells";

// Rolling deploy (ADR 0024 step 5.2): this web may get its answer from a machine still on the
// previous release, which answers with the old names.
describe("normalizeCastAnswer", () => {
  it("reads a previous release's cast answer under this release's names", () => {
    const old = {
      entryId: 12,
      source: "cast",
      spellId: "magic_missile",
      name: "Magic Missile",
      level: 1,
      casterLevel: 15,
      spell: {
        id: "magic_missile",
        name: { en: "Magic Missile" },
        class: "wizard",
      },
      consumedItem: { itemId: "diamond", name: "Diamond", remaining: 3 },
      attack: { d20Roll: 17, targetAc: 5, hit: true, baseThac0: 13 },
      untracked: [],
    } as unknown as CastResult;
    const got = normalizeCastAnswer(old);
    expect(got).toMatchObject({
      logEntryId: 12,
      action: "cast",
      spell: "Magic Missile",
      catalogueSpell: { id: "magic_missile", class: "wizard" },
      item: { itemId: "diamond", remaining: 3 },
      attack: { d20: 17, targetAc: 5, hit: true },
    });
  });

  it("leaves this release's answer as served (its compat keys aside)", () => {
    const answer = {
      action: "received",
      spellId: "sleep",
      spell: "Sleep",
      casterLevel: 5,
      item: { itemId: "gem", name: "Gem", remaining: 1 },
      attack: { d20: 9, targetAc: 4, hit: false, d20Roll: 9 },
      logEntryId: 3,
      catalogueSpell: { id: "sleep" },
      entryId: 3,
      source: "received",
      name: "Sleep",
      consumedItem: { itemId: "gem", name: "Gem", remaining: 1 },
    } as unknown as CastResult;
    const got = normalizeCastAnswer(answer);
    expect(got).toMatchObject({
      action: "received",
      spell: "Sleep",
      logEntryId: 3,
      catalogueSpell: { id: "sleep" },
      item: { itemId: "gem" },
      attack: { d20: 9 },
    });
  });
});

describe("normalizeExposureAnswer", () => {
  it("reads a previous release's exposure answer under this release's names", () => {
    const old = {
      entryId: 8,
      spellId: "cloudkill",
      name: "Cloudkill",
      casterLevel: 15,
      round: 2,
      onSelf: false,
      hpDelta: 0,
      damage: { total: 4, type: "poison" },
    } as unknown as ExposureResult;
    expect(normalizeExposureAnswer(old)).toMatchObject({
      logEntryId: 8,
      spell: "Cloudkill",
    });
  });
});
