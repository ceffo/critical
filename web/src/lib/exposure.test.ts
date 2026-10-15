// Display of a spell's exposure damage (ADR 0008 step 16): the dice text, the lookup rows, the
// journal entry and its card, the HP bar's last change. Every number is the server's.
import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import type {
  ExposureResult,
  SpellDamage,
  SpellExposure,
} from "@/types/spells";
import { exposureDamageText } from "./spellDice";
import { exposureRows, exposureText } from "./spellMode";
import { entryText } from "./logSummary";
import {
  cardHeader,
  cardModel,
  cardSource,
  isRollEntry,
  latestResult,
  liveModel,
  rowValue,
} from "./filCard";
import { lastHpChange } from "./moiLog";

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

const one = { groups: [], modifier: 1 };
const damage = (dice: SpellDamage["dice"], type: string): SpellDamage => ({
  caster_level: 15,
  dice,
  instances: one,
  rounds: one,
  type,
});
const cloud: SpellExposure = {
  damage: damage({ groups: [{ count: 1, sides: 10 }], modifier: 0 }, "poison"),
};
const wall: SpellExposure = {
  modes: [
    {
      key: "within_10_ft",
      damage: damage({ groups: [{ count: 2, sides: 4 }], modifier: 0 }, "fire"),
    },
    {
      key: "passing_through",
      damage: damage(
        { groups: [{ count: 2, sides: 6 }], modifier: 15 },
        "fire",
      ),
    },
  ],
};
const fog: SpellExposure = {
  damage: {
    ...damage({ groups: [], modifier: 0 }, "acid"),
    rounds: { groups: [], modifier: 4 },
    steps_per: "round",
    steps: [1, 2, 4, 8].map((n, i) => ({
      step: i + 1,
      dice: { groups: [], modifier: n },
    })),
  },
};

function entry(payload: unknown, over: Partial<LogEntry> = {}): LogEntry {
  return {
    id: 7,
    transactionId: 7,
    characterId: 1,
    round: 2,
    kind: "exposure",
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
    ...over,
  };
}

const payload = {
  spellId: "cloudkill",
  spell: "Cloudkill",
  casterLevel: 15,
  round: 2,
  onSelf: true,
  damage: {
    total: 7,
    type: "poison",
    dice: "1d10",
    instances: 1,
    rounds: 1,
    rolls: [{ formula: "1d10", dice: [{ sides: 10, value: 7 }], total: 7 }],
  },
  exposure: cloud,
};

describe("exposure dice text", () => {
  it("one damage, modes, and dice steps by round of exposure (en)", async () => {
    const t = await makeT("en");
    expect(exposureText(t, "cloudkill", cloud)).toBe(
      "1d10 poison per round of exposure",
    );
    expect(exposureText(t, "wall_of_fire", wall)).toBe(
      "Within 10 ft: 2d4 fire · Passing through: 2d6+15 fire",
    );
    expect(exposureText(t, "death_fog", fog)).toBe(
      "1, 2, 4, 8 acid by round of exposure, then 8 each round",
    );
    expect(exposureDamageText(t, fog.damage!)).toBe(
      exposureText(t, "death_fog", fog),
    );
  });

  it("is worded in French", async () => {
    const t = await makeT("fr");
    expect(exposureText(t, "cloudkill", cloud)).toBe(
      "1d10 poison par round d'exposition",
    );
    expect(exposureText(t, "wall_of_fire", wall)).toBe(
      "À 10 pi ou moins : 2d4 feu · En la traversant : 2d6+15 feu",
    );
  });

  it("the lookup has one row per exposure mode, or one, or none", async () => {
    const t = await makeT("en");
    expect(exposureRows(t, { id: "cloudkill", exposure: cloud })).toEqual([
      { key: "exposure", label: "Exposure", damage: cloud.damage },
    ]);
    expect(
      exposureRows(t, { id: "wall_of_fire", exposure: wall }).map(
        (r) => r.label,
      ),
    ).toEqual(["Exposure (Within 10 ft)", "Exposure (Passing through)"]);
    expect(exposureRows(t, { id: "fireball" })).toEqual([]);
  });
});

describe("exposure entries", () => {
  it("are rolls and the latest result", () => {
    const e = entry(payload);
    expect(isRollEntry(e)).toBe(true);
    expect(latestResult([e])).toBe(e);
  });

  it("word the journal row: on me or for the DM, with the mode", async () => {
    const t = await makeT("en");
    expect(entryText(entry(payload), t)).toBe(
      "Cloudkill exposure, round 2: 7 poison damage to me",
    );
    expect(
      entryText(
        entry({
          ...payload,
          spellId: "wall_of_fire",
          spell: "Wall of Fire",
          mode: "within_10_ft",
          onSelf: false,
        }),
        t,
      ),
    ).toBe("Wall of Fire exposure, round 2: 7 poison damage · Within 10 ft");
  });

  it("the row value: the HP the server applied on me, the total for the DM", () => {
    expect(
      rowValue(entry(payload, { hpDelta: -7, effectiveHpDelta: -7 })),
    ).toEqual({ text: "−7", tone: "damage" });
    expect(
      rowValue(
        entry(payload, { hpDelta: -7, effectiveHpDelta: 0, status: "voided" }),
      ),
    ).toEqual({
      text: "−7",
      tone: "damage",
    });
    expect(rowValue(entry({ ...payload, onSelf: false }))).toEqual({
      text: "7",
      tone: "accent",
    });
  });

  it("the card renders from the payload as the endpoint's result, with the served exposure", async () => {
    const t = await makeT("en");
    const model = cardModel(entry(payload), t);
    expect(model).toEqual({
      layout: "exposure",
      exposure: {
        entryId: 7,
        spellId: "cloudkill",
        name: "Cloudkill",
        casterLevel: 15,
        mode: undefined,
        round: 2,
        onSelf: true,
        hpDelta: 0,
        damage: {
          dice: "1d10",
          type: "poison",
          instances: 1,
          rounds: 1,
          rolls: payload.damage.rolls,
          total: 7,
          parts: undefined,
          stepsPer: undefined,
        },
        exposure: cloud,
      },
    });
    expect(cardHeader("exposure", model, entry(payload), t, "en").tag).toBe(
      "Exposure",
    );
    expect(
      cardHeader("exposure", model, entry(payload), t, "en").subtitle,
    ).toBe("Cloudkill");
  });

  it("a live exposure is the card until the log has caught up", () => {
    const result: ExposureResult = {
      entryId: 8,
      spellId: "cloudkill",
      name: "Cloudkill",
      casterLevel: 15,
      round: 3,
      onSelf: false,
      hpDelta: 0,
      damage: {
        dice: "1d10",
        type: "poison",
        instances: 1,
        rounds: 1,
        rolls: [],
        total: 4,
      },
    };
    const last = { kind: "exposure" as const, result };
    expect(liveModel(last)).toEqual({ layout: "exposure", exposure: result });
    expect(cardSource(last, [entry(payload)], null)).toMatchObject({
      kind: "live",
    });
  });

  it("an exposure on me is the HP bar's last change; one for the DM is not", () => {
    const onMe = entry(payload, { hpDelta: -7, effectiveHpDelta: -7 });
    expect(lastHpChange([onMe])).toEqual({
      delta: -7,
      source: "",
      exposure: { spellId: "cloudkill", spell: "Cloudkill" },
    });
    expect(lastHpChange([entry({ ...payload, onSelf: false })])).toBeNull();
    expect(
      lastHpChange([{ ...onMe, status: "voided", effectiveHpDelta: 0 }]),
    ).toBeNull();
  });

  it("an exposure cut by the HP floor shows the HP applied, on the card and the journal row", async () => {
    const t = await makeT("en");
    const floored = entry(
      {
        ...payload,
        damage: { ...payload.damage, total: 10 },
        hpRequested: -10,
        hpApplied: -2,
      },
      { hpDelta: -2, effectiveHpDelta: -2 },
    );
    const model = cardModel(floored, t);
    expect(model.layout === "exposure" && model.exposure.hpDelta).toBe(-2);
    expect(rowValue(floored)).toEqual({ text: "−2", tone: "damage" });
    expect(entryText(floored, t)).toBe(
      "Cloudkill exposure, round 2: 10 poison damage to me · −10 requested, stopped at the HP floor",
    );
  });

  it("Wall of Fire passing through names its mode in the journal, in French too (spell names outside the spellbook stay English)", async () => {
    const wall = entry({
      ...payload,
      spellId: "wall_of_fire",
      spell: "Wall of Fire",
      casterLevel: 11,
      mode: "passing_through",
      round: 1,
      onSelf: false,
      damage: { ...payload.damage, total: 19, type: "fire", dice: "2d6+level" },
    });
    expect(entryText(wall, await makeT("en"))).toBe(
      "Wall of Fire exposure, round 1: 19 fire damage · Passing through",
    );
    expect(entryText(wall, await makeT("fr"))).toBe(
      "Wall of Fire, exposition au round 1 : 19 dégâts (feu) · En la traversant",
    );
  });

  it("a cast entry from the journal keeps the exposure it served, to roll from after a reload", async () => {
    const t = await makeT("en");
    const cast = {
      ...entry({
        action: "cast",
        spellId: "cloudkill",
        spell: "Cloudkill",
        casterLevel: 15,
        exposure: cloud,
      }),
      kind: "cast" as const,
    };
    const model = cardModel(cast, t);
    expect(model.layout).toBe("cast");
    expect(model.layout === "cast" && model.cast.exposure).toEqual(cloud);
  });
});
