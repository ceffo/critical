import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry, LogKind } from "@/types/log";
import type { RollResult, SaveBreakdown } from "@/types/combat";
import type { CastResult } from "@/types/spells";
import {
  cardContent,
  cardEntryId,
  cardSource,
  lastResultEntry,
  latestResult,
  type LiveResult,
} from "./card";
import { dieFaces, dieTone } from "./dice";
import { entryCapped, filterEntries, isRolledEntry, rowValue } from "./entry";
import { attackDiceChips } from "./kinds/attack";
import { castInstances, type CastPayload } from "./kinds/cast";
import { saveView } from "./kinds/save";
import { KIND_VIEWS } from "./registry";
import { roundMarker } from "./rounds";
import { cardOf } from "./testing";
import { englishSpellName } from "@/i18n/spellName";
import { recordedNames } from "@/i18n/inventoryName";
import { undoButtonState } from "@/lib/undoButton";

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
const strCheck = {
  ability: "str",
  die: 20,
  roll: 15,
  abilityScore: 12,
  target: 16,
  adjustment: 0,
  adjustedTarget: 16,
  success: true,
  rolledSuccess: true,
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

const KINDS: [LogKind, unknown, string, Partial<LogEntry>?][] = [
  ["attack", attackPayload, "attack"],
  ["skill_check", profCheck, "skill"],
  ["ability_check", strCheck, "ability"],
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

describe("cardContent: every log kind renders from its payload", () => {
  it("covers every kind", () => {
    const kinds = new Set(KINDS.map(([k]) => k));
    expect(kinds.size).toBe(KINDS.length);
    expect(kinds.size).toBe(24);
  });

  it.each(KINDS)("%s → %s", async (kind, payload, layout, over) => {
    const t = await makeT("en");
    const c = cardOf(entry(kind, payload, over), t);
    expect(c.view).toBe(KIND_VIEWS[kind]);
    expect(c.payload).not.toBeNull();
    // A kind with a layout has its card body; the others show the journal's text.
    expect(c.view?.card !== undefined).toBe(
      layout !== "info" || kind === "hp_change",
    );
    // Worded from the payload, never an empty line.
    expect(c.text).not.toBe("");
  });

  it("skill check: the payload as recorded, header names the skill (en, fr)", async () => {
    const e = entry("skill_check", profCheck, { id: 9 });
    const en = cardOf(e, await makeT("en"));
    expect(en.payload).toEqual({ ...profCheck, logEntryId: 9 });
    expect(en.subtitle).toBe("Musical instrument");
    const fr = cardOf(e, await makeT("fr"), "fr");
    expect(fr.tag).toBe("Compétence");
    expect(fr.subtitle).toBe("Instrument de musique");
    expect(
      cardOf(entry("skill_check", { id: "x" }), await makeT("en")).payload,
    ).toBeNull();
  });

  it("ability check: the payload as recorded, header names the ability (en, fr)", async () => {
    const e = entry("ability_check", strCheck, { id: 9 });
    const en = cardOf(e, await makeT("en"));
    expect(en.payload).toEqual({ ...strCheck, logEntryId: 9 });
    expect(en.tag).toBe("Ability");
    expect(en.subtitle).toBe("Strength");
    const fr = cardOf(e, await makeT("fr"), "fr");
    expect(fr.tag).toBe("Caractéristique");
    expect(fr.subtitle).toBe("Force");
    expect(
      cardOf(entry("ability_check", { ability: "str" }), await makeT("en"))
        .payload,
    ).toBeNull();
  });

  it("attack: the payload as recorded, with its entry id for the correction", async () => {
    const c = cardOf(
      entry("attack", attackPayload, { id: 42 }),
      await makeT("en"),
    );
    expect(c.payload).toEqual({ ...attackPayload, logEntryId: 42 });
  });

  it("attack: a corrected attack shows the latest amend's payload", async () => {
    const corrected = {
      ...attackPayload,
      targetAc: 7,
      hit: false,
      rolledTargetAc: 5,
      rolledHit: true,
    };
    const c = cardOf(
      entry("attack", attackPayload, {
        effectivePayload: corrected,
        status: "amended",
      }),
      await makeT("en"),
    );
    expect((c.payload as RollResult).targetAc).toBe(7);
    expect((c.payload as RollResult).hit).toBe(false);
  });

  it("save: d20, target, modifiers, adjustment and the server's roll total, adjusted target and verdict", async () => {
    const c = cardOf(entry("save", savePayload, { id: 9 }), await makeT("en"));
    expect(saveView(c.payload as SaveBreakdown)).toEqual({
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
    });
  });

  it("save: a payload without modifiers still renders", async () => {
    const c = cardOf(
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
    expect(saveView(c.payload as SaveBreakdown).modifiers).toEqual([]);
  });

  it("save: without the roll total and adjusted target the server fills (ADR 0006 step 8), it is not read as a save", async () => {
    const c = cardOf(
      entry("save", {
        category: "breath",
        d20Roll: 4,
        target: 12,
        passed: false,
        adjustment: 0,
      }),
      await makeT("en"),
    );
    expect(c.payload).toBeNull();
  });

  it("cast and item: the payloads as recorded", async () => {
    const t = await makeT("en");
    expect(cardOf(entry("cast", castPayload), t).payload).toEqual({
      breakdown: castPayload,
      answer: null,
    });
    expect(cardOf(entry("item_use", usePayload, { id: 5 }), t).payload).toEqual(
      { ...usePayload, logEntryId: 5 },
    );
  });

  it("initiative: the round and its d10", async () => {
    expect(
      cardOf(entry("initiative", { round: 3, d10: 7 }), await makeT("en"))
        .payload,
    ).toEqual({ round: 3, d10: 7 });
  });

  it("an HP change carries its value (damage red, heal green) and the cap", async () => {
    const t = await makeT("en");
    const damage = entry("hp_change", null, {
      hpDelta: -6,
      effectiveHpDelta: -6,
      effectiveSummary: "orc axe",
    });
    expect(cardOf(damage, t).text).toBe("orc axe");
    expect(rowValue(damage)).toEqual({ text: "−6", tone: "damage" });
    expect(entryCapped(damage)).toBeNull();
    const heal = entry(
      "hp_change",
      { hpRequested: 12, hpApplied: 5 },
      { hpDelta: 5, effectiveHpDelta: 5, effectiveSummary: "potion" },
    );
    expect(rowValue(heal)).toEqual({ text: "+5", tone: "heal" });
    expect(entryCapped(heal)).toEqual({ hpRequested: 12, hpApplied: 5 });
    // A voided change keeps its original delta (struck through) and no cap note.
    const voided = entry(
      "hp_change",
      { hpRequested: 12, hpApplied: 5 },
      { hpDelta: 5, effectiveHpDelta: 0, status: "voided" },
    );
    expect(rowValue(voided)).toEqual({ text: "+5", tone: "heal" });
    expect(entryCapped(voided)).toBeNull();
  });

  it("non-HP entries without a layout are worded in the player's language with no value", async () => {
    const fr = await makeT("fr");
    const e = entry("round_advance", {
      action: "next_round",
      fromRound: 2,
      toRound: 3,
    });
    const c = cardOf(e, fr, "fr");
    expect(c.view?.card).toBeUndefined();
    expect(rowValue(e)).toBeNull();
    expect(c.text).toMatch(/2 → 3/);
  });

  it("an unreadable roll payload falls back to the server's text", async () => {
    const c = cardOf(
      entry(
        "attack",
        { legacy: true },
        { effectiveSummary: "Old attack line" },
      ),
      await makeT("en"),
    );
    expect(c.payload).toBeNull();
    expect(c.text).toBe("Old attack line");
  });
});

describe("card header", () => {
  it.each([
    ["attack", attackPayload, "Attack", "Long Sword +2"],
    ["save", savePayload, "Save", "Spells"],
    ["cast", castPayload, "Cast", "Magic Missile"],
    ["item_use", usePayload, "Item", "Wand of lightning"],
    ["initiative", { round: 3, d10: 7 }, "Initiative", "Round 3"],
    ["buff_change", { action: "dismiss", buff: "Haste" }, "Effect", ""],
  ] as const)(
    "%s: tag %s, subtitle %s",
    async (kind, payload, tag, subtitle) => {
      const t = await makeT("en");
      const h = cardOf(entry(kind, payload), t);
      expect(h.tag).toBe(tag);
      expect(h.subtitle).toBe(subtitle);
      expect(h.when).toMatch(/^R3 · \d\d:\d\d/);
    },
  );

  it("an attack with ammunition names both; round 0 reads out of combat (fr)", async () => {
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
    const h = cardOf(e, fr, "fr");
    expect(h.subtitle).toBe("Short Bow +2 (Arrows +3)");
    expect(h.when).toMatch(/^Hors combat · /);
  });

  it("a live result whose entry the log has not loaded yet has no time", async () => {
    const t = await makeT("en");
    const h = cardContent(
      {
        kind: "live",
        last: { kind: "attack", result: attackPayload },
        entry: null,
      },
      t,
      "en",
      englishSpellName,
      recordedNames,
    );
    expect(h.when).toBe("");
    expect(h.tag).toBe("Attack");
  });
});

describe("cardSource: latest result by default, a selected entry read-only", () => {
  const hp = entry("hp_change", null, { id: 4 });
  const save = entry("save", savePayload, { id: 3, roll: true });
  const attack = entry("attack", attackPayload, { id: 2, roll: true });
  const entries = [hp, save, attack]; // newest first
  const liveSave: LiveResult = {
    kind: "save",
    result: { ...savePayload, logEntryId: 3 } as SaveBreakdown,
  };

  it("the latest result is the newest attack, save, cast or item use, voided included", () => {
    expect(latestResult(entries)?.id).toBe(3);
    expect(latestResult([{ ...save, status: "voided" }, attack])?.id).toBe(3);
    expect(latestResult([hp])).toBeNull();
    // An ability check is a roll kind (ADR 0028 step 5): it takes the card.
    const check = entry("ability_check", strCheck, { id: 5, roll: true });
    expect(latestResult([hp, check, save])?.id).toBe(5);
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

describe("live results", () => {
  const live = async (last: LiveResult) =>
    cardContent(
      { kind: "live", last, entry: null },
      await makeT("en"),
      "en",
      englishSpellName,
      recordedNames,
    );

  it("a save response is its log entry's payload: the live card reads it like the journal's", async () => {
    const r = { ...savePayload, logEntryId: 9 } as SaveBreakdown;
    expect((await live({ kind: "save", result: r })).payload).toEqual(
      cardOf(entry("save", savePayload, { id: 9 }), await makeT("en")).payload,
    );
  });

  it("each live kind is its log kind's view; a cast keeps the full response", async () => {
    const cast = {
      logEntryId: 3,
      spellId: "sleep",
      spell: "Sleep",
    } as CastResult;
    expect((await live({ kind: "cast", result: cast })).payload).toEqual({
      breakdown: cast,
      answer: cast,
    });
    expect((await live({ kind: "attack", result: attackPayload })).view).toBe(
      KIND_VIEWS.attack,
    );
    expect(
      (await live({ kind: "item_use", result: { ...usePayload } as never }))
        .view,
    ).toBe(KIND_VIEWS.item_use);
    expect(
      (await live({ kind: "skill_check", result: bardCheck as never })).payload,
    ).toEqual(bardCheck);
    const ability = await live({
      kind: "ability_check",
      result: strCheck as never,
    });
    expect(ability.payload).toEqual(strCheck);
    expect(ability.tag).toBe("Ability");
  });
});

describe("journal: rolls filter and row values", () => {
  it("rolls are attacks, saves, item uses, initiative and casts that rolled", () => {
    expect(isRolledEntry(entry("attack", attackPayload))).toBe(true);
    expect(isRolledEntry(entry("save", savePayload))).toBe(true);
    expect(isRolledEntry(entry("item_use", usePayload))).toBe(true);
    expect(isRolledEntry(entry("initiative", { round: 1, d10: 3 }))).toBe(true);
    expect(isRolledEntry(entry("skill_check", bardCheck))).toBe(true);
    expect(isRolledEntry(entry("ability_check", strCheck))).toBe(true);
    expect(isRolledEntry(entry("dice", diceRoll))).toBe(true);
    expect(isRolledEntry(entry("cast", castPayload))).toBe(true);
    expect(
      isRolledEntry(
        entry("cast", {
          ...castPayload,
          damage: undefined,
          attack: { d20: 9, targetAc: 4, hit: true },
        }),
      ),
    ).toBe(true);
    expect(
      isRolledEntry(
        entry("cast", {
          action: "cast",
          spellId: "haste",
          spell: "Haste",
          casterLevel: 15,
        }),
      ),
    ).toBe(false);
    expect(isRolledEntry(entry("hp_change", null))).toBe(false);
    expect(
      isRolledEntry(entry("buff_change", { action: "dismiss", buff: "Haste" })),
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
    [entry("ability_check", strCheck), { text: "15", tone: "pass" }],
    [
      entry("ability_check", { ...strCheck, success: false }),
      { text: "15", tone: "fail" },
    ],
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
        roll: true,
        effectivePayload: corrected,
        status: "amended",
      }),
    ];
    const fr = await makeT("fr");
    expect(undoButtonState(3, before, false, fr).label).toBe(
      "Annuler : Correction CA 5 → 7 (Ctrl+Z / ⌘Z)",
    );

    const after = [
      step(3, "voided"),
      entry("attack", rolled, { id: 2, roll: true }),
    ];
    const source = cardSource(null, after, null);
    expect(source).toEqual({ kind: "entry", entry: after[1], latest: true });
    const m = cardOf(after[1], fr, "fr").payload as RollResult;
    expect(m.targetAc).toBe(5);
    expect(m.rolledTargetAc).toBeUndefined();
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
    const m = cardOf(
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
    expect(attackDiceChips(m.payload as RollResult)).toEqual([
      { value: 6, tone: "max" },
      { value: 1, tone: "min" },
    ]);
  });

  it("attack, old entry without the dice: no chips, the card falls back to the total", async () => {
    const t = await makeT("en");
    const m = cardOf(entry("attack", attackPayload), t);
    expect(attackDiceChips(m.payload as RollResult)).toBeNull();
  });

  it("cast, new entry: the instances with their dice, plus the save", async () => {
    const t = await makeT("en");
    const m = cardOf(
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
    const cast = (m.payload as CastPayload).breakdown;
    expect(castInstances(cast)).toEqual([
      { dice: [{ value: 6, tone: "max" }], total: 7 },
      { dice: [{ value: 3, tone: "plain" }], total: 4 },
    ]);
    const sum = castInstances(cast)?.reduce((n, i) => n + i.total, 0);
    expect(sum).toBe(cast.damage?.total);
    expect(cast.save).toEqual({ type: "spell", effect: "half" });
  });

  it("cast with damage parts: each instance lists its parts and their save", async () => {
    const t = await makeT("en");
    const d4 = (a: number, b: number) => [
      { sides: 4, value: a },
      { sides: 4, value: b },
    ];
    const burst = { formula: "level", dice: [], total: 10 };
    const m = cardOf(
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
    const cast = (m.payload as CastPayload).breakdown;
    const max = { value: 4, tone: "max" };
    const min = { value: 1, tone: "min" };
    expect(castInstances(cast)?.[1]).toEqual({
      dice: [max, min],
      total: 15,
      parts: [
        { dice: [max, min], total: 5, save: "none" },
        { dice: [], total: 10, save: "negates" },
      ],
    });
    expect(cast.damage?.totalOnSave).toBe(11);
  });

  it("cast with dice steps: each roll keeps its step, a step without damage has no dice", async () => {
    const t = await makeT("en");
    const m = cardOf(
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
    const cast = (m.payload as CastPayload).breakdown;
    const got = castInstances(cast);
    expect(got?.map((i) => i.step)).toEqual([1, 2, 3]);
    expect(got?.[0]).toEqual({ dice: [], total: 0, step: 1 });
    expect(cast.damage?.stepsPer).toBe("round");
  });

  it("cast, old entry without the dice: no instances, still a cast card", async () => {
    const t = await makeT("en");
    const m = cardOf(
      entry("cast", { ...castBase, damage: { total: 24, type: "fire" } }),
      t,
    );
    const cast = (m.payload as CastPayload).breakdown;
    expect(castInstances(cast)).toBeNull();
    expect(cast.damage?.total).toBe(24);
  });
});

describe("header: last result", () => {
  const hp = entry("hp_change", null, { id: 5 });
  const dice = entry("dice", diceRoll, { id: 4 });
  const save = entry("save", savePayload, { id: 3, roll: true });
  const attack = entry("attack", attackPayload, { id: 2, roll: true });

  it("is the newest result or free dice roll still in force", () => {
    expect(lastResultEntry([hp, dice, save, attack])?.id).toBe(4);
    expect(
      lastResultEntry([hp, { ...dice, status: "voided" }, save, attack])?.id,
    ).toBe(3);
    expect(lastResultEntry([hp])).toBeNull();
    // ADR 0028 step 5: an ability check takes it; once popped, the roll before it is back.
    const check = entry("ability_check", strCheck, { id: 7, roll: true });
    const skill = entry("skill_check", profCheck, { id: 6, roll: true });
    expect(lastResultEntry([check, skill, save])?.id).toBe(7);
    expect(
      lastResultEntry([{ ...check, status: "voided" }, skill, save])?.id,
    ).toBe(6);
  });
});

describe("roundMarker (ADR 0027 step 9.3)", () => {
  it("is T3 in a round, an icon out of combat, the full label in the tooltip", async () => {
    const fr = await makeT("fr");
    expect(roundMarker(fr, 3, true)).toEqual({
      short: "T3",
      full: "Tour 3 · en cours",
    });
    expect(roundMarker(fr, 2, false)).toEqual({ short: "T2", full: "Tour 2" });
    expect(roundMarker(fr, 0, true)).toEqual({
      short: null,
      full: "Hors combat · en cours",
    });
    const en = await makeT("en");
    expect(roundMarker(en, 0, false)).toEqual({
      short: null,
      full: "Out of combat",
    });
  });
});
