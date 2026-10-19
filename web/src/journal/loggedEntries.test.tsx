// Entries logged before the cast and exposure payloads became the answer (no level, untracked,
// buff effects, damage rolls) must still render in the journal and the result card, and an old
// exposure entry must still offer the next round.
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import { castInstances, castKind, type CastPayload } from "./kinds/cast";
import { exposureKind } from "./kinds/exposure";

async function i18n() {
  const inst = i18next.createInstance();
  await inst.init({
    lng: "en",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst;
}

function entry(kind: LogEntry["kind"], payload: unknown): LogEntry {
  return {
    id: 7,
    transactionId: 7,
    characterId: 1,
    round: 2,
    kind,
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
    createdAt: "2026-10-15T21:31:00Z",
  };
}

const oldCast = {
  action: "cast",
  spellId: "fireball",
  spell: "Fireball",
  casterLevel: 5,
  damage: { total: 24, type: "fire" },
  buff: { name: "Fireball", applied: true },
  attack: { d20: 14, targetAc: 5, hit: true },
};

const oldExposure = {
  spellId: "cloudkill",
  spell: "Cloudkill",
  casterLevel: 15,
  round: 2,
  onSelf: false,
  damage: { total: 7, type: "poison" },
  exposure: {
    damage: {
      caster_level: 15,
      dice: { groups: [{ count: 1, sides: 10 }], modifier: 0 },
      instances: { groups: [], modifier: 1 },
      rounds: { groups: [], modifier: 1 },
      type: "poison",
    },
  },
};

describe("entries logged before the payload was the answer", () => {
  it("an old cast renders in the journal with its d20 and its total", async () => {
    const inst = await i18n();
    const e = entry("cast", oldCast);
    const p = castKind.decode(e) as CastPayload;
    expect(castInstances(p.breakdown)).toBeNull();
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={inst}>
        {castKind.card?.({ payload: p, entry: e, text: "", interactive: true })}
      </I18nextProvider>,
    );
    expect(html).toContain("Fireball");
    expect(html).toContain('aria-label="d20: 14"'); // the die
    expect(html).toContain(">14<");
    expect(html).toContain("24");
  });

  it("a cast logged with the retired d20Roll alias beside d20 still renders its die", async () => {
    const inst = await i18n();
    const e = entry("cast", {
      ...oldCast,
      attack: { d20: 17, d20Roll: 17, targetAc: 5, hit: true },
    });
    const p = castKind.decode(e) as CastPayload;
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={inst}>
        {castKind.card?.({ payload: p, entry: e, text: "", interactive: true })}
      </I18nextProvider>,
    );
    expect(html).toContain('aria-label="d20: 17"');
  });

  it("an old exposure renders in the journal and offers the next round", async () => {
    const inst = await i18n();
    const e = entry("exposure", oldExposure);
    const p = exposureKind.decode(e);
    if (!p) throw new Error("not an exposure");
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={inst}>
        {exposureKind.card?.({
          payload: p,
          entry: e,
          text: "",
          interactive: true,
        })}
      </I18nextProvider>,
    );
    expect(html).toContain("Cloudkill");
    expect(html).toContain("7");
    expect(html).toContain('data-testid="cast-exposure"');
    expect(html).toMatch(/value="3"/); // the round after the logged 2
  });
});
