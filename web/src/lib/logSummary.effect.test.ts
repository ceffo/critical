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

function castEntry(payload: unknown): LogEntry {
  return {
    id: 1,
    transactionId: 1,
    characterId: 1,
    round: 0,
    kind: "cast",
    summary: "",
    hpDelta: 0,
    effectiveHpDelta: 0,
    note: "",
    effectiveSummary: "",
    status: "active",
    payload,
    effectivePayload: payload,
    snapshots: [],
    corrections: [],
    createdAt: "2026-10-16T00:00:00Z",
  };
}

// ADR 0004 step 19: a spell added as an effect from the lookup is a cast entry with action
// "effect"; the journal words it with the Bard's level, and cast / received keep their wording.
describe("cast entry actions", () => {
  const buff = { name: "Strength", rounds: 900, strGain: 4, applied: true };
  const payload = (action: string) => ({
    action,
    spellId: "strength",
    spell: "Strength",
    casterLevel: 15,
    buff,
  });

  it("words each action in English", async () => {
    const t = await makeT("en");
    expect(entryText(castEntry(payload("effect")), t)).toMatch(
      /^Strength added as an effect \(level 15\): /,
    );
    expect(entryText(castEntry(payload("received")), t)).toMatch(
      /^Strength cast on me \(caster level 15\): /,
    );
    expect(entryText(castEntry(payload("cast")), t)).toMatch(
      /^Cast Strength: /,
    );
  });

  it("words an effect in French", async () => {
    const t = await makeT("fr");
    expect(entryText(castEntry(payload("effect")), t)).toMatch(
      /^Effet ajouté : Strength \(niveau 15\)/,
    );
    // The keys the cast box, the result heading and the refusal use (LookupPanel,
    // CastResultView, the API error).
    expect(
      t("lookup.addEffectLabel", { name: "Image miroir", level: 15 }),
    ).toBe("Ajouter Image miroir comme effet au niveau 15");
    expect(
      t("result.cast.effectHeading", { name: "Image miroir", casterLevel: 15 }),
    ).toBe("Effet ajouté : Image miroir, niveau 15");
    expect(t("apiError.spell_not_an_effect")).toBe(
      "Ce sort n'ajoute aucun effet à suivre.",
    );
  });
});
