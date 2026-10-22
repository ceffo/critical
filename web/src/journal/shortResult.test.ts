import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry, LogKind } from "@/types/log";
import { shortResult } from "./entry";
import { recordedNames } from "@/i18n/inventoryName";

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

const attack = {
  d20Roll: 17,
  isCrit: false,
  hit: true,
  totalDamage: 8,
  weaponName: "Long Sword +2",
  targetAc: 5,
};
const damage = { total: 12, type: "fire" };
const skill = {
  id: "musical_instrument",
  kind: "proficiency",
  die: 20,
  roll: 17,
  target: 16,
  success: false,
};
const ability = {
  ability: "str",
  die: 20,
  roll: 15,
  target: 16,
  adjustment: 0,
  adjustedTarget: 16,
  success: true,
};

type Want = { value: string | null; en: string; fr: string };

/** One short wording per log kind (every LogKind is a key: a new kind fails to compile here). */
const CASES: Record<LogKind, { e: LogEntry } & Want> = {
  attack: {
    e: entry("attack", attack),
    value: "17",
    en: "Attack · hit",
    fr: "Attaque · touché",
  },
  save: {
    e: entry("save", {
      category: "spell",
      d20Roll: 12,
      passed: true,
      rollTotal: 12,
      adjustedTarget: 14,
      margin: 0,
      target: 14,
    }),
    value: "12",
    en: "Spells save · passed",
    fr: "Sauvegarde Sorts · réussie",
  },
  skill_check: {
    e: entry("skill_check", skill),
    value: "17",
    en: "Musical Instrument · failed",
    fr: "Instrument de musique · échec",
  },
  ability_check: {
    e: entry("ability_check", ability),
    value: "15",
    en: "STR · success",
    fr: "FOR · réussi",
  },
  dice: {
    e: entry("dice", { formula: "2d6+1", groups: [], modifier: 1, total: 9 }),
    value: "9",
    en: "2d6+1",
    fr: "2d6+1",
  },
  cast: {
    e: entry("cast", {
      action: "cast",
      spellId: "fireball",
      spell: "Fireball",
      casterLevel: 5,
      damage,
    }),
    value: "12",
    en: "Fireball",
    fr: "Boule de feu",
  },
  exposure: {
    e: entry("exposure", {
      spellId: "fireball",
      spell: "Fireball",
      casterLevel: 5,
      round: 2,
      onSelf: false,
      damage,
    }),
    value: "12",
    en: "Fireball",
    fr: "Boule de feu",
  },
  item_use: {
    e: entry("item_use", {
      name: "Wand",
      itemId: "wand",
      remaining: 3,
      dice: "1d6",
      total: 4,
    }),
    value: "4",
    en: "Wand",
    fr: "Wand",
  },
  hp_change: {
    e: entry("hp_change", null, { hpDelta: -8, effectiveHpDelta: -8 }),
    value: "−8",
    en: "Damage",
    fr: "Dégâts",
  },
  psp_change: {
    e: entry("psp_change", null, { pspDelta: 12 }),
    value: "+12",
    en: "PSPs recovered",
    fr: "PFP récupérés",
  },
  psp_recovery: {
    e: entry(
      "psp_recovery",
      { hours: 3, activity: "resting", perHour: 6 },
      { pspDelta: 18 },
    ),
    value: "+18",
    en: "PSPs recovered",
    fr: "PFP récupérés",
  },
  psionic_pick: {
    e: entry("psionic_pick", {
      before: { picks: [] },
      after: { picks: [{ kind: "discipline", id: "telepathy" }] },
    }),
    value: null,
    en: "Known powers",
    fr: "Pouvoirs connus",
  },
  initiative: {
    e: entry("initiative", { round: 3, d10: 7 }),
    value: "7",
    en: "Initiative",
    fr: "Initiative",
  },
  initiative_correct: {
    e: entry("initiative_correct", {
      round: 2,
      d10: 3,
      before: 8,
      after: 4,
      modifiers: [{ source: "spell", value: 1, spellId: "magic_missile" }],
      cause: { source: "spell", value: 1, spellId: "magic_missile" },
    }),
    value: "4",
    en: "Initiative",
    fr: "Initiative",
  },
  buff_change: {
    e: entry("buff_change", null),
    value: null,
    en: "Effect",
    fr: "Effet",
  },
  round_advance: {
    e: entry("round_advance", null),
    value: null,
    en: "Round",
    fr: "Tour",
  },
  round_reset: {
    e: entry("round_reset", null),
    value: null,
    en: "Round",
    fr: "Tour",
  },
  round_set: {
    e: entry("round_set", null),
    value: null,
    en: "Round",
    fr: "Tour",
  },
  time_passed: {
    e: entry("time_passed", null),
    value: null,
    en: "Time",
    fr: "Temps",
  },
  item_restock: {
    e: entry("item_restock", null),
    value: null,
    en: "Item",
    fr: "Objet",
  },
  item_add: {
    e: entry("item_add", null),
    value: null,
    en: "Item",
    fr: "Objet",
  },
  item_edit: {
    e: entry("item_edit", null),
    value: null,
    en: "Item",
    fr: "Objet",
  },
  item_remove: {
    e: entry("item_remove", null),
    value: null,
    en: "Item",
    fr: "Objet",
  },
  weapon_add: {
    e: entry("weapon_add", null),
    value: null,
    en: "Weapon",
    fr: "Arme",
  },
  weapon_edit: {
    e: entry("weapon_edit", null),
    value: null,
    en: "Weapon",
    fr: "Arme",
  },
  weapon_remove: {
    e: entry("weapon_remove", null),
    value: null,
    en: "Weapon",
    fr: "Arme",
  },
  equipment_state: {
    e: entry("equipment_state", {
      itemId: "chain",
      name: "Chain mail",
      category: "armour",
      before: "carried",
      after: "worn",
    }),
    value: null,
    en: "Equipment",
    fr: "Équipement",
  },
  xp_add: { e: entry("xp_add", null), value: null, en: "XP", fr: "XP" },
  xp_edit: { e: entry("xp_edit", null), value: null, en: "XP", fr: "XP" },
  xp_remove: { e: entry("xp_remove", null), value: null, en: "XP", fr: "XP" },
  level_up: {
    e: entry("level_up", {
      level: 16,
      hitDie: 0,
      rolled: null,
      counted: null,
      conAdjustment: 0,
      gain: 2,
      fixed: true,
      manual: false,
      hpMaxBefore: 85,
      hpMaxAfter: 87,
    }),
    value: "+2",
    en: "Level",
    fr: "Niveau",
  },
  spell_learn: {
    e: entry("spell_learn", {
      spellId: "sleep",
      spellLevel: 1,
      roll: 37,
      chance: 65,
      learned: true,
      characterLevel: 1,
      retryLevel: 0,
    }),
    value: null,
    en: "Learn",
    fr: "Apprentissage",
  },
  spell_add: {
    e: entry("spell_add", {
      spellId: "sleep",
      spellLevel: 1,
      origin: "given",
    }),
    value: null,
    en: "Spellbook",
    fr: "Grimoire",
  },
  turn_undead: {
    e: entry("turn_undead", {
      undead: "ghoul",
      clericLevel: 3,
      target: 10,
      d20: 12,
      success: true,
      effect: "turn",
      affected: [3, 4],
      destroyed: 0,
      turned: 7,
    }),
    value: "12",
    en: "Turning Ghoul · turned",
    fr: "Renvoi Goule · repoussés",
  },
  skill_points: {
    e: entry("skill_points", {
      before: { open_locks: 30 },
      after: { open_locks: 45 },
    }),
    value: null,
    en: "Skills",
    fr: "Talents",
  },
  proficiencies: {
    e: entry("proficiencies", {
      before: { nonweapon: [{ id: "tumbling", slots: 1 }] },
      after: { nonweapon: [{ id: "tumbling", slots: 2 }] },
    }),
    value: null,
    en: "Proficiencies",
    fr: "Compétences",
  },
  skill_armor: {
    e: entry("skill_armor", { before: "none", after: "elven_chain" }),
    value: null,
    en: "Armour",
    fr: "Armure",
  },
  class_switch: {
    e: entry("class_switch", {
      from: "fighter",
      to: "mage",
      leftAtLevel: 6,
      xp: 21000,
    }),
    value: null,
    en: "Class",
    fr: "Classe",
  },
  ac_override: {
    e: entry("ac_override", { before: null, after: { value: 4 } }),
    value: null,
    en: "AC",
    fr: "CA",
  },
  spell_prep: {
    e: entry("spell_prep", null),
    value: null,
    en: "Prepare",
    fr: "Préparation",
  },
  long_rest: {
    e: entry("long_rest", null),
    value: null,
    en: "Rest",
    fr: "Repos",
  },
  correction: {
    e: entry("correction", null),
    value: null,
    en: "Correction",
    fr: "Correction",
  },
};

describe("shortResult", () => {
  for (const [kind, c] of Object.entries(CASES)) {
    it(`${kind}: value, what and verdict, en and fr`, async () => {
      for (const lng of ["en", "fr"] as const) {
        const t = await makeT(lng);
        const names = (_id: string | undefined, name: string) => name;
        const r = shortResult(
          c.e,
          t,
          (id, name) =>
            lng === "fr" && id === "fireball" ? "Boule de feu" : (name ?? ""),
          { ...recordedNames, item: names },
        );
        const line = r.verdict ? `${r.what} · ${r.verdict.text}` : r.what;
        expect(r.value?.text ?? null).toBe(c.value);
        expect(line.toLowerCase()).toBe(c[lng].toLowerCase());
        if (r.verdict)
          expect(r.verdict.text).toBe(r.verdict.text.toLowerCase());
      }
    });
  }

  // ADR 0030 step 16.4: D* destroys the 2d6 (the row value) and turns the 2d4 (in the verdict).
  it("words a D* turn with both counts, en and fr", async () => {
    const e = entry("turn_undead", {
      undead: "skeleton",
      clericLevel: 8,
      outcome: "destroy_more",
      success: true,
      effect: "destroy",
      affected: [2, 3],
      more: [1, 2],
      destroyed: 5,
      turned: 3,
    });
    const want = {
      en: "Turning Skeleton · destroyed, 3 turned",
      fr: "Renvoi Squelette · détruits, 3 repoussés",
    };
    for (const lng of ["en", "fr"] as const) {
      const t = await makeT(lng);
      const r = shortResult(e, t, (_id, name) => name ?? "", {
        ...recordedNames,
        item: (_id: string | undefined, name: string) => name,
      });
      expect(r.value?.text).toBe("5");
      expect(`${r.what} · ${r.verdict?.text}`).toBe(want[lng]);
    }
  });

  it("colours the verdict: hit green, miss red, crit highlight; unsettled has none", async () => {
    const t = await makeT("en");
    const run = (a: object) =>
      shortResult(
        entry("attack", { ...attack, ...a }),
        t,
        (_i, n) => n ?? "",
        recordedNames,
      ).verdict;
    expect(run({})).toEqual({ text: "hit", tone: "pass" });
    expect(run({ hit: false })).toEqual({ text: "missed", tone: "fail" });
    expect(run({ isCrit: true })).toEqual({ text: "critical", tone: "crit" });
    expect(run({ acKnown: false, targetAc: null })).toBeNull();
  });
});
