import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry, LogKind } from "@/types/log";
import type { RollResult } from "@/types/combat";
import type { CastResult } from "@/types/spells";
import {
  attackDiceChips,
  castInstances,
  cardEntryId,
  cardHeader,
  cardModel,
  cardSource,
  dieFaces,
  dieTone,
  filterEntries,
  isRollEntry,
  lastResultChip,
  lastResultEntry,
  latestResult,
  liveModel,
  rowValue,
  saveFromRoll,
  turnMarker,
  type CardModel,
  type LiveResult,
} from "./filCard";
import { undoButtonState } from "./undoButton";

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
  kind: LogKind,
  payload: unknown,
  over: Partial<LogEntry> = {},
): LogEntry {
  return {
    id: 1,
    transactionId: 1,
    characterId: 1,
    round: 3,
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
    ...over,
  };
}

const attackPayload = {
  d20Roll: 14,
  d100CritRoll: 80,
  isCrit: false,
  critMultiplier: 1,
  critPlaceholder: false,
  baseThac0: 13,
  modifiers: [{ kind: "strength", detail: "12", value: 0 }],
  effectiveThac0: 11,
  targetACNeeded: -3,
  hit: true,
  hitMargin: 8,
  baseDamage: 6,
  damageDice: "1d8",
  spellsBuff: 1,
  weaponBuff: 2,
  multipliers: 1,
  strengthBonus: 0,
  bonusDamage: 0,
  damageTerms: [],
  totalDamage: 8,
  weaponName: "Long Sword +2",
  targetAc: 5,
  targetSize: "sm",
} as unknown as RollResult;

const savePayload = {
  category: "spell",
  d20Roll: 12,
  modifiers: [{ kind: "buff", detail: "Prayer", value: 1 }],
  target: 14,
  rolledPassed: false,
  adjustment: 2,
  adjustmentReason: "Gaia",
  rollTotal: 13,
  adjustedTarget: 12,
  total: 15,
  passed: true,
  margin: 1,
};

const castPayload = {
  action: "cast",
  spellId: "magic_missile",
  spell: "Magic Missile",
  casterLevel: 15,
  damage: { total: 17, type: "force" },
};

const usePayload = {
  itemId: "wand_lightning",
  name: "Wand of lightning",
  remaining: 3,
  maxCount: 0,
  dice: "6d6",
  rolls: [6, 1, 3, 4, 2, 5],
  total: 21,
  save: "rod_staff_wand",
  notes: "",
};

/** Every log kind with a payload as its service records it, and the card layout it gets. */
const profCheck = {
  id: "musical_instrument",
  kind: "proficiency",
  die: 20,
  roll: 17,
  target: 16,
  success: false,
  ability: "dex",
  abilityScore: 17,
  modifier: -1,
};
const bardCheck = {
  id: "climb_walls",
  kind: "bardSkill",
  die: 100,
  roll: 42,
  target: 85,
  success: true,
  modifier: 0,
};

const diceRoll = {
  formula: "2d6-d4+3",
  groups: [
    { sides: 6, rolls: [2, 5], negative: false, subtotal: 7 },
    { sides: 4, rolls: [3], negative: true, subtotal: -3 },
  ],
  modifier: 3,
  total: 7,
};

const KINDS: [LogKind, unknown, CardModel["layout"], Partial<LogEntry>?][] = [
  ["attack", attackPayload, "attack"],
  ["skill_check", profCheck, "skill"],
  ["dice", diceRoll, "dice"],
  ["save", savePayload, "save"],
  ["cast", castPayload, "cast"],
  ["item_use", usePayload, "item"],
  ["initiative", { round: 3, d10: 7 }, "initiative"],
  [
    "hp_change",
    { hpRequested: 12, hpApplied: 5 },
    "info",
    {
      hpDelta: 5,
      effectiveHpDelta: 5,
      summary: "e2e potion",
      effectiveSummary: "e2e potion",
    },
  ],
  ["buff_change", { action: "activate", buff: "Haste", rounds: 5 }, "info"],
  ["round_advance", { action: "next_round", fromRound: 2, toRound: 3 }, "info"],
  ["round_reset", { action: "end_combat", fromRound: 4, toRound: 0 }, "info"],
  ["round_set", { action: "set_round", fromRound: 4, toRound: 9 }, "info"],
  [
    "time_passed",
    { action: "pass_time", hours: 1, rounds: 60, fromRound: 3, toRound: 0 },
    "info",
  ],
  [
    "item_restock",
    { itemId: "arrows", name: "Arrows", type: "arrow", added: 2, count: 12 },
    "info",
  ],
  ["item_add", { name: "Rope", count: 1 }, "info"],
  [
    "item_edit",
    {
      name: "Rope",
      count: 1,
      changes: [{ field: "count", before: "1", after: "2" }],
    },
    "info",
  ],
  ["item_remove", { name: "Rope", count: 1 }, "info"],
  ["weapon_add", { name: "Dagger" }, "info"],
  [
    "weapon_edit",
    {
      name: "Dagger",
      changes: [{ field: "name", before: "Dagger", after: "Knife" }],
    },
    "info",
  ],
  ["weapon_remove", { name: "Dagger" }, "info"],
  [
    "spell_prep",
    { spells: [{ id: "sleep", name: "Sleep", count: 2 }] },
    "info",
  ],
  ["long_rest", { slotsReset: 9, itemsRefilled: ["holy_water"] }, "info"],
  [
    "correction",
    { entryId: 2, entryKind: "attack", field: "targetAc", from: 5, to: 7 },
    "info",
  ],
];

describe("cardModel: every log kind renders from its payload", () => {
  it("covers every kind", () => {
    const kinds = new Set(KINDS.map(([k]) => k));
    expect(kinds.size).toBe(KINDS.length);
    // LogKind has 23 members (types/log.ts); a new kind must be added here.
    expect(kinds.size).toBe(23);
  });

  it.each(KINDS)("%s → %s", async (kind, payload, layout, over) => {
    const t = await makeT("en");
    const m = cardModel(entry(kind, payload, over), t);
    expect(m.layout).toBe(layout);
    if (m.layout === "info") {
      // Worded from the payload, never an empty line.
      expect(m.info.text).not.toBe("");
    }
  });

  it("skill check: the payload as recorded, header names the skill (en, fr)", async () => {
    const e = entry("skill_check", profCheck, { id: 9 });
    const m = cardModel(e, await makeT("en"));
    expect(m).toEqual({
      layout: "skill",
      check: { ...profCheck, logEntryId: 9 },
    });
    expect(
      cardHeader("skill_check", m, e, await makeT("en"), "en").subtitle,
    ).toBe("Musical instrument");
    const fr = cardHeader("skill_check", m, e, await makeT("fr"), "fr");
    expect(fr.tag).toBe("Compétence");
    expect(fr.subtitle).toBe("Instrument de musique");
    expect(
      cardModel(entry("skill_check", { id: "x" }), await makeT("en")).layout,
    ).toBe("info");
  });

  it("attack: the payload as recorded, with its entry id for the correction", async () => {
    const m = cardModel(
      entry("attack", attackPayload, { id: 42 }),
      await makeT("en"),
    );
    expect(m).toEqual({
      layout: "attack",
      roll: { ...attackPayload, logEntryId: 42 },
    });
  });

  it("attack: a corrected attack shows the latest amend's payload", async () => {
    const corrected = {
      ...attackPayload,
      targetAc: 7,
      hit: false,
      rolledTargetAc: 5,
      rolledHit: true,
    };
    const m = cardModel(
      entry("attack", attackPayload, {
        effectivePayload: corrected,
        status: "amended",
      }),
      await makeT("en"),
    );
    expect(m.layout === "attack" && m.roll.targetAc).toBe(7);
    expect(m.layout === "attack" && m.roll.hit).toBe(false);
  });

  it("save: d20, target, modifiers, adjustment and the server's roll total, adjusted target and verdict", async () => {
    const m = cardModel(
      entry("save", savePayload, { id: 9 }),
      await makeT("en"),
    );
    expect(m).toEqual({
      layout: "save",
      save: {
        category: "spell",
        d20: 12,
        target: 14,
        modifiers: [{ kind: "buff", detail: "Prayer", value: 1 }],
        adjustment: 2,
        adjustmentReason: "Gaia",
        rollTotal: 13,
        adjustedTarget: 12,
        passed: true,
        margin: 1,
        logEntryId: 9,
      },
    });
  });

  it("save: a payload without modifiers still renders", async () => {
    const m = cardModel(
      entry("save", {
        category: "breath",
        d20Roll: 4,
        target: 12,
        passed: false,
        adjustment: 0,
        rollTotal: 4,
        adjustedTarget: 12,
        margin: -8,
      }),
      await makeT("en"),
    );
    expect(m.layout === "save" && m.save.modifiers).toEqual([]);
  });

  it("save: without the roll total and adjusted target the server fills (ADR 0006 step 8), it is not read as a save", async () => {
    const m = cardModel(
      entry("save", {
        category: "breath",
        d20Roll: 4,
        target: 12,
        passed: false,
        adjustment: 0,
      }),
      await makeT("en"),
    );
    expect(m.layout).toBe("info");
  });

  it("cast and item: the payloads as recorded", async () => {
    const t = await makeT("en");
    expect(cardModel(entry("cast", castPayload), t)).toEqual({
      layout: "cast",
      cast: castPayload,
    });
    expect(cardModel(entry("item_use", usePayload, { id: 5 }), t)).toEqual({
      layout: "item",
      item: { ...usePayload, logEntryId: 5 },
    });
  });

  it("initiative: the turn and its d10", async () => {
    expect(
      cardModel(entry("initiative", { round: 3, d10: 7 }), await makeT("en")),
    ).toEqual({
      layout: "initiative",
      round: 3,
      d10: 7,
    });
  });

  it("info: an HP change carries its value (damage red, heal green) and the cap", async () => {
    const t = await makeT("en");
    const damage = cardModel(
      entry("hp_change", null, {
        hpDelta: -6,
        effectiveHpDelta: -6,
        effectiveSummary: "orc axe",
      }),
      t,
    );
    expect(damage).toEqual({
      layout: "info",
      info: {
        text: "orc axe",
        value: { amount: -6, tone: "damage" },
        capped: null,
      },
    });
    const heal = cardModel(
      entry(
        "hp_change",
        { hpRequested: 12, hpApplied: 5 },
        { hpDelta: 5, effectiveHpDelta: 5, effectiveSummary: "potion" },
      ),
      t,
    );
    expect(heal.layout === "info" && heal.info.value).toEqual({
      amount: 5,
      tone: "heal",
    });
    expect(heal.layout === "info" && heal.info.capped).toEqual({
      hpRequested: 12,
      hpApplied: 5,
    });
    // A voided change keeps its original delta (struck through) and no cap note.
    const voided = cardModel(
      entry(
        "hp_change",
        { hpRequested: 12, hpApplied: 5 },
        { hpDelta: 5, effectiveHpDelta: 0, status: "voided" },
      ),
      t,
    );
    expect(voided.layout === "info" && voided.info.value).toEqual({
      amount: 5,
      tone: "heal",
    });
    expect(voided.layout === "info" && voided.info.capped).toBeNull();
  });

  it("info: non-HP entries are worded in the player's language with no value", async () => {
    const fr = await makeT("fr");
    const m = cardModel(
      entry("round_advance", {
        action: "next_round",
        fromRound: 2,
        toRound: 3,
      }),
      fr,
    );
    expect(m.layout === "info" && m.info.value).toBeNull();
    expect(m.layout === "info" && m.info.text).toMatch(/2 → 3/);
  });

  it("an unreadable roll payload falls back to info with the server's text", async () => {
    const m = cardModel(
      entry(
        "attack",
        { legacy: true },
        { effectiveSummary: "Old attack line" },
      ),
      await makeT("en"),
    );
    expect(m).toEqual({
      layout: "info",
      info: { text: "Old attack line", value: null, capped: null },
    });
  });
});

describe("cardHeader", () => {
  it.each([
    ["attack", attackPayload, "Attack", "Long Sword +2"],
    ["save", savePayload, "Save", "Spells"],
    ["cast", castPayload, "Cast", "Magic Missile"],
    ["item_use", usePayload, "Item", "Wand of lightning"],
    ["initiative", { round: 3, d10: 7 }, "Initiative", "Turn 3"],
    ["buff_change", { action: "dismiss", buff: "Haste" }, "Buff", ""],
  ] as const)(
    "%s: tag %s, subtitle %s",
    async (kind, payload, tag, subtitle) => {
      const t = await makeT("en");
      const e = entry(kind, payload);
      const h = cardHeader(kind, cardModel(e, t), e, t, "en");
      expect(h.tag).toBe(tag);
      expect(h.subtitle).toBe(subtitle);
      expect(h.when).toMatch(/^T3 · \d\d:\d\d/);
    },
  );

  it("an attack with ammunition names both; turn 0 reads out of combat (fr)", async () => {
    const fr = await makeT("fr");
    const e = entry(
      "attack",
      {
        ...attackPayload,
        weaponName: "Short Bow +2",
        ammo: { itemId: "a", name: "Arrows +3", magic: 3, remaining: 4 },
      },
      { round: 0 },
    );
    const h = cardHeader("attack", cardModel(e, fr), e, fr, "fr");
    expect(h.subtitle).toBe("Short Bow +2 (Arrows +3)");
    expect(h.when).toMatch(/^Hors combat · /);
  });

  it("a live result whose entry the log has not loaded yet has no time", async () => {
    const t = await makeT("en");
    const h = cardHeader(
      "attack",
      { layout: "attack", roll: attackPayload },
      null,
      t,
      "en",
    );
    expect(h.when).toBe("");
  });
});

describe("cardSource: latest result by default, a selected entry read-only", () => {
  const hp = entry("hp_change", null, { id: 4 });
  const save = entry("save", savePayload, { id: 3 });
  const attack = entry("attack", attackPayload, { id: 2 });
  const entries = [hp, save, attack]; // newest first
  const liveSave: LiveResult = {
    kind: "save",
    category: "spell",
    result: { ...attackPayload, logEntryId: 3 },
  };

  it("the latest result is the newest attack, save, cast or item use, voided included", () => {
    expect(latestResult(entries)?.id).toBe(3);
    expect(latestResult([{ ...save, status: "voided" }, attack])?.id).toBe(3);
    expect(latestResult([hp])).toBeNull();
  });

  it("without a selection: this session's roll while it is the latest", () => {
    expect(cardSource(liveSave, entries, null)).toEqual({
      kind: "live",
      last: liveSave,
      entry: save,
    });
  });

  it("a roll newer than the loaded log shows at once (the log catches up after)", () => {
    const newer: LiveResult = {
      kind: "attack",
      result: { ...attackPayload, logEntryId: 10 },
    };
    expect(cardSource(newer, entries, null)).toEqual({
      kind: "live",
      last: newer,
      entry: null,
    });
  });

  it("after a reload (no live roll): the newest result entry from its payload", () => {
    expect(cardSource(null, entries, null)).toEqual({
      kind: "entry",
      entry: save,
      latest: true,
    });
  });

  it("a live roll older than the newest result entry gives way to it", () => {
    const old: LiveResult = {
      kind: "attack",
      result: { ...attackPayload, logEntryId: 2 },
    };
    expect(cardSource(old, entries, null)).toEqual({
      kind: "entry",
      entry: save,
      latest: true,
    });
  });

  it("a selected older entry is read-only, the latest selected is the latest", () => {
    expect(cardSource(liveSave, entries, 2)).toEqual({
      kind: "entry",
      entry: attack,
      latest: false,
    });
    expect(cardSource(liveSave, entries, 4)).toEqual({
      kind: "entry",
      entry: hp,
      latest: false,
    });
    expect(cardSource(liveSave, entries, 3).kind).toBe("live");
    // A selection that left the log (popped and reloaded) falls back to the latest.
    expect(cardSource(liveSave, entries, 99).kind).toBe("live");
  });

  it("nothing to show", () => {
    expect(cardSource(null, [hp], null)).toEqual({ kind: "empty" });
  });

  it("the highlighted journal row follows the card", () => {
    expect(cardEntryId(cardSource(liveSave, entries, null))).toBe(3);
    expect(cardEntryId(cardSource(liveSave, entries, 4))).toBe(4);
    expect(cardEntryId(cardSource(null, [hp], null))).toBeNull();
  });
});

describe("liveModel", () => {
  it("a save response reads like its log entry: the sheet's target, the roll total and the adjusted target", () => {
    const r = {
      ...attackPayload,
      d20Roll: 12,
      baseThac0: 14,
      effectiveThac0: 12,
      modifiers: [{ kind: "buff", detail: "Prayer", value: 1 }],
      hit: true,
      hitMargin: 1,
      adjustment: 2,
      adjustmentReason: "Gaia",
      saveTotal: 15,
      rollTotal: 13,
      adjustedTarget: 12,
      logEntryId: 9,
    } as RollResult;
    expect(saveFromRoll("spell", r)).toEqual(
      (
        cardModel(
          entry("save", savePayload, { id: 9 }),
          ((k: string) => k) as never,
        ) as { save: unknown }
      ).save,
    );
  });

  it("each live kind maps to its layout; a cast keeps the full response", () => {
    const cast = { entryId: 3, spellId: "sleep", name: "Sleep" } as CastResult;
    expect(liveModel({ kind: "cast", result: cast })).toEqual({
      layout: "castLive",
      result: cast,
    });
    expect(liveModel({ kind: "attack", result: attackPayload }).layout).toBe(
      "attack",
    );
    expect(
      liveModel({ kind: "item", result: { ...usePayload } as never }).layout,
    ).toBe("item");
    expect(liveModel({ kind: "skill", result: bardCheck as never })).toEqual({
      layout: "skill",
      check: bardCheck,
    });
  });
});

describe("journal: rolls filter and row values", () => {
  it("rolls are attacks, saves, item uses, initiative and casts that rolled", () => {
    expect(isRollEntry(entry("attack", attackPayload))).toBe(true);
    expect(isRollEntry(entry("save", savePayload))).toBe(true);
    expect(isRollEntry(entry("item_use", usePayload))).toBe(true);
    expect(isRollEntry(entry("initiative", { round: 1, d10: 3 }))).toBe(true);
    expect(isRollEntry(entry("skill_check", bardCheck))).toBe(true);
    expect(isRollEntry(entry("dice", diceRoll))).toBe(true);
    expect(isRollEntry(entry("cast", castPayload))).toBe(true);
    expect(
      isRollEntry(
        entry("cast", {
          ...castPayload,
          damage: undefined,
          attack: { d20: 9, targetAc: 4, hit: true },
        }),
      ),
    ).toBe(true);
    expect(
      isRollEntry(
        entry("cast", {
          action: "cast",
          spellId: "haste",
          spell: "Haste",
          casterLevel: 15,
        }),
      ),
    ).toBe(false);
    expect(isRollEntry(entry("hp_change", null))).toBe(false);
    expect(
      isRollEntry(entry("buff_change", { action: "dismiss", buff: "Haste" })),
    ).toBe(false);
  });

  it("filters", () => {
    const list = [
      entry("hp_change", null, { id: 2 }),
      entry("attack", attackPayload, { id: 1 }),
    ];
    expect(filterEntries(list, "all")).toHaveLength(2);
    expect(filterEntries(list, "rolls").map((e) => e.id)).toEqual([1]);
  });

  it.each([
    [
      entry("hp_change", null, { hpDelta: -6, effectiveHpDelta: -6 }),
      { text: "−6", tone: "damage" },
    ],
    [
      entry("hp_change", null, { hpDelta: 4, effectiveHpDelta: 4 }),
      { text: "+4", tone: "heal" },
    ],
    [entry("attack", attackPayload), { text: "8", tone: "ink" }],
    // Gold only where the payload says crit (ADR 0026 step 5.8).
    [
      entry("attack", { ...attackPayload, isCrit: true }),
      { text: "8", tone: "crit" },
    ],
    [
      entry("attack", { ...attackPayload, hit: false }),
      { text: "–", tone: "muted" },
    ],
    // Against an unknown AC: "?" until the GM settles it, then the verdict's outcome.
    [
      entry("attack", {
        ...attackPayload,
        hit: false,
        targetAc: null,
        acKnown: false,
      }),
      { text: "?", tone: "muted" },
    ],
    [
      entry("attack", {
        ...attackPayload,
        targetAc: null,
        acKnown: false,
        verdict: "hit",
      }),
      { text: "8", tone: "ink" },
    ],
    [entry("save", savePayload), { text: "12", tone: "pass" }],
    [entry("item_use", usePayload), { text: "21", tone: "ink" }],
    [entry("item_use", { ...usePayload, dice: "", rolls: [], total: 0 }), null],
    [entry("initiative", { round: 3, d10: 7 }), { text: "7", tone: "ink" }],
    [entry("skill_check", profCheck), { text: "17", tone: "fail" }],
    [entry("skill_check", bardCheck), { text: "42", tone: "pass" }],
    [entry("dice", diceRoll), { text: "7", tone: "ink" }],
    [entry("cast", castPayload), { text: "17", tone: "ink" }],
    [entry("buff_change", { action: "dismiss", buff: "Haste" }), null],
  ])("row value %#", (e, want) => {
    expect(rowValue(e)).toEqual(want);
  });
});

describe("dice chips", () => {
  it("reads the faces of one dice group", () => {
    expect(dieFaces("6d6")).toBe(6);
    expect(dieFaces("1d8+2")).toBe(8);
    expect(dieFaces("d20")).toBe(20);
    expect(dieFaces("2d6+1d4")).toBeNull();
  });

  it("marks the maximum and a 1", () => {
    expect(dieTone(6, 6)).toBe("max");
    expect(dieTone(1, 6)).toBe("min");
    expect(dieTone(3, 6)).toBe("plain");
    expect(dieTone(6, null)).toBe("plain");
  });
});

// ADR 0010 step 6: popping a correction while the Rolls filter hides correction rows. The journal
// shows only the roll; the card (which reads the whole log, the live roll dropped by the pop) shows
// the roll's payload from the log, and the Undo tooltip still names the correction on top.
describe("a popped correction under the Rolls filter", () => {
  const rolled = { ...attackPayload, targetAc: 5 };
  const corrected = {
    ...attackPayload,
    targetAc: 7,
    rolledTargetAc: 5,
    rolledHit: true,
  };
  const step = (id: number, status: LogEntry["status"]) =>
    entry(
      "correction",
      { entryId: 2, entryKind: "attack", field: "targetAc", from: 5, to: 7 },
      { id, status },
    );

  it("the filter hides the correction rows, not the roll", () => {
    const log = [
      step(3, "active"),
      entry("attack", rolled, { id: 2, effectivePayload: corrected }),
    ];
    expect(filterEntries(log, "rolls").map((e) => e.id)).toEqual([2]);
  });

  it("after the pop the card reads the log's roll payload; the tooltip is in French", async () => {
    const before = [
      step(3, "active"),
      entry("attack", rolled, {
        id: 2,
        effectivePayload: corrected,
        status: "amended",
      }),
    ];
    const fr = await makeT("fr");
    expect(undoButtonState(3, before, false, fr).label).toBe(
      "Annuler : Correction CA 5 → 7 (Ctrl+Z / ⌘Z)",
    );

    const after = [step(3, "voided"), entry("attack", rolled, { id: 2 })];
    const source = cardSource(null, after, null);
    expect(source).toEqual({ kind: "entry", entry: after[1], latest: true });
    const m = cardModel(after[1], fr);
    expect(m.layout === "attack" && m.roll.targetAc).toBe(5);
    expect(m.layout === "attack" && m.roll.rolledTargetAc).toBeUndefined();
    expect(undoButtonState(2, after, false, fr).label).toMatch(
      /^Annuler : .*CA 5/,
    );
  });
});

describe("die chips from the payload", () => {
  const castBase = {
    action: "cast",
    spellId: "fireball",
    spell: "Fireball",
    casterLevel: 5,
  };

  it("attack, new entry: one chip per recorded die, max and 1 marked", async () => {
    const t = await makeT("en");
    const m = cardModel(
      entry("attack", {
        ...attackPayload,
        damageDice: "2d6",
        damageRolls: [
          { sides: 6, value: 6 },
          { sides: 6, value: 1 },
        ],
      }),
      t,
    );
    if (m.layout !== "attack") throw new Error("not an attack card");
    expect(attackDiceChips(m.roll)).toEqual([
      { value: 6, tone: "max" },
      { value: 1, tone: "min" },
    ]);
  });

  it("attack, old entry without the dice: no chips, the card falls back to the total", async () => {
    const t = await makeT("en");
    const m = cardModel(entry("attack", attackPayload), t);
    if (m.layout !== "attack") throw new Error("not an attack card");
    expect(attackDiceChips(m.roll)).toBeNull();
  });

  it("cast, new entry: the instances with their dice, plus the save", async () => {
    const t = await makeT("en");
    const m = cardModel(
      entry("cast", {
        ...castBase,
        damage: {
          total: 11,
          type: "force",
          dice: "1d6+1",
          instances: 2,
          rounds: 1,
          rolls: [
            { formula: "1d6+1", dice: [{ sides: 6, value: 6 }], total: 7 },
            { formula: "1d6+1", dice: [{ sides: 6, value: 3 }], total: 4 },
          ],
        },
        save: { type: "spell", effect: "half" },
      }),
      t,
    );
    if (m.layout !== "cast") throw new Error("not a cast card");
    expect(castInstances(m.cast)).toEqual([
      { dice: [{ value: 6, tone: "max" }], total: 7 },
      { dice: [{ value: 3, tone: "plain" }], total: 4 },
    ]);
    const sum = castInstances(m.cast)?.reduce((n, i) => n + i.total, 0);
    expect(sum).toBe(m.cast.damage?.total);
    expect(m.cast.save).toEqual({ type: "spell", effect: "half" });
  });

  it("cast with damage parts: each instance lists its parts and their save", async () => {
    const t = await makeT("en");
    const d4 = (a: number, b: number) => [
      { sides: 4, value: a },
      { sides: 4, value: b },
    ];
    const burst = { formula: "level", dice: [], total: 10 };
    const m = cardModel(
      entry("cast", {
        ...castBase,
        damage: {
          total: 31,
          totalOnSave: 11,
          type: "force",
          dice: "2d4+level",
          instances: 2,
          rounds: 1,
          parts: [
            { formula: "2d4", save: "none" },
            { formula: "level", save: "negates" },
          ],
          rolls: [
            {
              formula: "2d4+level",
              dice: d4(3, 3),
              total: 16,
              parts: [{ formula: "2d4", dice: d4(3, 3), total: 6 }, burst],
            },
            {
              formula: "2d4+level",
              dice: d4(4, 1),
              total: 15,
              parts: [{ formula: "2d4", dice: d4(4, 1), total: 5 }, burst],
            },
          ],
        },
      }),
      t,
    );
    if (m.layout !== "cast") throw new Error("not a cast card");
    const max = { value: 4, tone: "max" };
    const min = { value: 1, tone: "min" };
    expect(castInstances(m.cast)?.[1]).toEqual({
      dice: [max, min],
      total: 15,
      parts: [
        { dice: [max, min], total: 5, save: "none" },
        { dice: [], total: 10, save: "negates" },
      ],
    });
    expect(m.cast.damage?.totalOnSave).toBe(11);
  });

  it("cast with dice steps: each roll keeps its step, a step without damage has no dice", async () => {
    const t = await makeT("en");
    const m = cardModel(
      entry("cast", {
        ...castBase,
        damage: {
          total: 5,
          type: "fire",
          dice: "",
          instances: 1,
          rounds: 3,
          stepsPer: "round",
          rolls: [
            { formula: "0", dice: [], total: 0, step: 1 },
            {
              formula: "leveld2",
              dice: [
                { sides: 2, value: 2 },
                { sides: 2, value: 1 },
              ],
              total: 3,
              step: 2,
            },
            {
              formula: "leveld2",
              dice: [
                { sides: 2, value: 1 },
                { sides: 2, value: 1 },
              ],
              total: 2,
              step: 3,
            },
          ],
        },
      }),
      t,
    );
    if (m.layout !== "cast") throw new Error("not a cast card");
    const got = castInstances(m.cast);
    expect(got?.map((i) => i.step)).toEqual([1, 2, 3]);
    expect(got?.[0]).toEqual({ dice: [], total: 0, step: 1 });
    expect(m.cast.damage?.stepsPer).toBe("round");
  });

  it("cast, old entry without the dice: no instances, still a cast card", async () => {
    const t = await makeT("en");
    const m = cardModel(
      entry("cast", { ...castBase, damage: { total: 24, type: "fire" } }),
      t,
    );
    if (m.layout !== "cast") throw new Error("not a cast card");
    expect(castInstances(m.cast)).toBeNull();
    expect(m.cast.damage?.total).toBe(24);
  });
});

describe("header: last result", () => {
  const hp = entry("hp_change", null, { id: 5 });
  const dice = entry("dice", diceRoll, { id: 4 });
  const save = entry("save", savePayload, { id: 3 });
  const attack = entry("attack", attackPayload, { id: 2 });

  it("is the newest result or free dice roll still in force", () => {
    expect(lastResultEntry([hp, dice, save, attack])?.id).toBe(4);
    expect(
      lastResultEntry([hp, { ...dice, status: "voided" }, save, attack])?.id,
    ).toBe(3);
    expect(lastResultEntry([hp])).toBeNull();
  });

  it("an attack shows its d20 in the chip, any other result its row value", () => {
    expect(lastResultChip(attack)).toEqual({ text: "14", tone: "muted" });
    expect(lastResultChip(save)).toEqual({ text: "12", tone: "pass" });
    expect(lastResultChip(dice)).toEqual({ text: "7", tone: "ink" });
  });
});

describe("turnMarker (ADR 0027 step 9.3)", () => {
  it("is T3 in a turn, an icon out of combat, the full label in the tooltip", async () => {
    const fr = await makeT("fr");
    expect(turnMarker(fr, 3, true)).toEqual({
      short: "T3",
      full: "Tour 3 · en cours",
    });
    expect(turnMarker(fr, 2, false)).toEqual({ short: "T2", full: "Tour 2" });
    expect(turnMarker(fr, 0, true)).toEqual({
      short: null,
      full: "Hors combat · en cours",
    });
    const en = await makeT("en");
    expect(turnMarker(en, 0, false)).toEqual({
      short: null,
      full: "Out of combat",
    });
  });
});
