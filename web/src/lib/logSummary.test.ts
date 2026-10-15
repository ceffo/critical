import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import { abilityText, entryText } from "./logSummary";

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
    transactionId: 1,
    characterId: 1,
    round: 0,
    kind,
    summary,
    hpDelta: 0,
    effectiveHpDelta: 0,
    note: "",
    effectiveSummary: summary,
    status: "active",
    payload,
    effectivePayload: payload,
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
    [
      {
        action: "edit",
        buff: "Ward",
        fromRounds: 5,
        rounds: 5,
        note: "fire −50%",
        fromNote: "",
        noteChanged: true,
      },
      "Ward: note “fire −50%”",
    ],
    [
      {
        action: "edit",
        buff: "Ward",
        fromRounds: 5,
        rounds: 5,
        note: "",
        fromNote: "x",
        noteChanged: true,
      },
      "Ward: note removed",
    ],
    [
      {
        action: "edit",
        buff: "Ward",
        fromRounds: 5,
        rounds: 3,
        note: "x",
        noteChanged: true,
      },
      "Ward: 5 rd → 3 rd (note changed)",
    ],
    [
      { action: "save_template", buff: "Hammer form", rounds: 10 },
      "Saved buff Hammer form (10 rd)",
    ],
    [
      { action: "delete_template", buff: "Hammer form" },
      "Deleted saved buff Hammer form",
    ],
    [{ action: "next_round", fromRound: 0, toRound: 1 }, "Turn 0 → 1"],
    [
      {
        action: "next_round",
        fromRound: 4,
        toRound: 5,
        expired: ["Strength", "Haste"],
      },
      "Turn 4 → 5 — expired: Strength, Haste",
    ],
  ])("renders %j", async (payload, want) => {
    const t = await makeT("en");
    const kind =
      payload.action === "next_round" ? "round_advance" : "buff_change";
    expect(entryText(entry(kind, payload), t)).toBe(want);
  });

  it("translates template and note log texts to French", async () => {
    const t = await makeT("fr");
    const text = (p: object) => entryText(entry("buff_change", p), t);
    expect(text({ action: "save_template", buff: "Marteau", rounds: 10 })).toBe(
      "Bonus Marteau enregistré (10 rd.)",
    );
    expect(text({ action: "delete_template", buff: "Marteau" })).toBe(
      "Bonus enregistré Marteau supprimé",
    );
    expect(
      text({
        action: "edit",
        buff: "Ward",
        fromRounds: 5,
        rounds: 5,
        note: "feu",
        noteChanged: true,
      }),
    ).toBe("Ward : note « feu »");
    expect(
      text({
        action: "edit",
        buff: "Ward",
        fromRounds: 5,
        rounds: 5,
        fromNote: "x",
        noteChanged: true,
      }),
    ).toBe("Ward : note retirée");
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

  it.each([
    [
      {
        hit: true,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 7,
        weaponName: "Long Sword +2",
        targetAc: 5,
      },
      "en",
      "Long Sword +2 vs AC 5: hit, 7 damage",
    ],
    [
      {
        hit: false,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 0,
        weaponName: "Short Bow +2",
        targetAc: 2,
        ammo: {
          itemId: "arrows_plus3",
          name: "Arrows +3",
          magic: 3,
          remaining: 1,
        },
      },
      "en",
      "Short Bow +2 (Arrows +3) vs AC 2: miss",
    ],
    [
      {
        hit: true,
        isCrit: true,
        critMultiplier: 2,
        totalDamage: 14,
        weaponName: "Long Sword +2",
        targetAc: 5,
      },
      "fr",
      // Logged before the multiplier d100 existed: the line without the rolls.
      "Long Sword +2 contre CA 5 : critique ×2, 14 dégâts",
    ],
    [
      {
        hit: true,
        isCrit: true,
        critMultiplier: 3,
        totalDamage: 21,
        weaponName: "Long Sword +2",
        targetAc: 5,
        d100CritRoll: 7,
        d100MultiplierRoll: 63,
      },
      "en",
      "Long Sword +2 vs AC 5: critical (check 7 · multiplier 63 → ×3), 21 damage",
    ],
    [
      {
        hit: true,
        isCrit: true,
        critMultiplier: 1.5,
        totalDamage: 10,
        weaponName: "Long Sword +2",
        targetAc: 5,
        d100CritRoll: 7,
        d100MultiplierRoll: 63,
      },
      "fr",
      "Long Sword +2 contre CA 5 : critique (test 7 · multiplicateur 63 → ×1,5), 10 dégâts",
    ],
  ])("renders attack %j in %s", async (payload, lng, want) => {
    const t = await makeT(lng);
    expect(entryText(entry("attack", payload, "server"), t)).toBe(want);
  });

  it.each([
    [
      "en",
      {
        hit: true,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 7,
        weaponName: "Long Sword +2",
        targetAc: 7,
        rolledTargetAc: 5,
        rolledHit: false,
        rolledCrit: false,
      },
      "Long Sword +2 vs AC 7: hit, 7 damage (AC 5 → 7: MISS → HIT)",
    ],
    [
      "en",
      {
        hit: false,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 0,
        weaponName: "Long Sword +2",
        targetAc: 3,
        rolledTargetAc: 6,
        rolledHit: true,
        rolledCrit: true,
      },
      "Long Sword +2 vs AC 3: miss (AC 6 → 3: CRIT → MISS)",
    ],
    [
      "fr",
      {
        hit: true,
        isCrit: true,
        critMultiplier: 1.5,
        totalDamage: 10,
        weaponName: "Long Sword +2",
        targetAc: 10,
        d100CritRoll: 3,
        d100MultiplierRoll: 77,
        rolledTargetAc: -10,
        rolledHit: false,
        rolledCrit: false,
      },
      "Long Sword +2 contre CA 10 : critique (test 3 · multiplicateur 77 → ×1,5), 10 dégâts (CA -10 → 10 : RATÉ → CRITIQUE)",
    ],
  ])(
    "renders a corrected attack in %s from the effective payload",
    async (lng, effective, want) => {
      const t = await makeT(lng);
      const rolled = {
        hit: false,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 0,
        weaponName: "Long Sword +2",
        targetAc: 5,
      };
      expect(
        entryText(
          { ...entry("attack", rolled, "server"), effectivePayload: effective },
          t,
        ),
      ).toBe(want);
    },
  );

  it.each([
    ["en", {}, "Long Sword +2 vs unknown AC: hits AC -2"],
    ["fr", {}, "Long Sword +2 contre CA inconnue : touche CA -2"],
    [
      "en",
      { verdict: "hit", hit: true, totalDamage: 7 },
      "Long Sword +2 vs unknown AC: hit (GM), 7 damage",
    ],
    ["fr", { verdict: "miss" }, "Long Sword +2 contre CA inconnue : raté (MJ)"],
  ])(
    "renders an attack against an unknown AC in %s (%j)",
    async (lng, extra, want) => {
      const t = await makeT(lng);
      const payload = {
        hit: false,
        isCrit: false,
        critMultiplier: 1,
        totalDamage: 7,
        weaponName: "Long Sword +2",
        targetAc: null,
        acKnown: false,
        targetACNeeded: -2,
        hitsAc: -2,
        critsFromAc: 40,
        ...extra,
      };
      expect(entryText(entry("attack", payload, "server"), t)).toBe(want);
    },
  );

  it("falls back to the summary for an attack without a payload", async () => {
    const t = await makeT("en");
    expect(entryText(entry("attack", {}, "old attack"), t)).toBe("old attack");
  });
});

describe("entryText for saves", () => {
  const rolled = {
    category: "spell",
    d20Roll: 12,
    target: 14,
    passed: false,
    rolledPassed: false,
    adjustment: 0,
    margin: -2,
  };

  it.each([
    ["rolled", rolled, "Save vs Spells: d20 12 vs 14, FAILED"],
    [
      "adjusted with a reason",
      {
        ...rolled,
        adjustment: 4,
        adjustmentReason: "Gaia fire",
        passed: true,
        margin: 2,
        previousPassed: false,
      },
      "Save vs Spells: d20 12 vs 14, adjusted +4 Gaia fire: FAILED → PASSED",
    ],
    [
      "adjusted without a reason",
      {
        ...rolled,
        d20Roll: 15,
        adjustment: -2,
        passed: false,
        previousPassed: true,
      },
      "Save vs Spells: d20 15 vs 14, adjusted −2: PASSED → FAILED",
    ],
    [
      "adjustment removed",
      { ...rolled, previousPassed: true },
      "Save vs Spells: d20 12 vs 14, adjustment removed: PASSED → FAILED",
    ],
  ])("%s", async (_name, payload, want) => {
    const t = await makeT("en");
    expect(entryText(entry("save", payload), t)).toBe(want);
  });

  it("reads the effective payload, not the original", async () => {
    const t = await makeT("en");
    const e = {
      ...entry("save", rolled),
      effectivePayload: {
        ...rolled,
        adjustment: 4,
        passed: true,
        previousPassed: false,
      },
    };
    expect(entryText(e, t)).toBe(
      "Save vs Spells: d20 12 vs 14, adjusted +4: FAILED → PASSED",
    );
  });

  it("renders in French", async () => {
    const t = await makeT("fr");
    expect(
      entryText(
        entry("save", {
          ...rolled,
          adjustment: 4,
          adjustmentReason: "Gaia feu",
          passed: true,
          previousPassed: false,
        }),
        t,
      ),
    ).toBe(
      "Sauvegarde contre Sorts : d20 12 contre 14, ajustée +4 Gaia feu : ÉCHOUÉE → RÉUSSIE",
    );
  });

  it("falls back to the server summary on an unknown payload", async () => {
    const t = await makeT("en");
    expect(entryText(entry("save", {}), t)).toBe("server summary");
  });
});

describe("time_passed entries", () => {
  const payload = {
    action: "pass_time",
    hours: 1.5,
    rounds: 90,
    fromRound: 4,
    toRound: 0,
    expired: ["Bless"],
  };
  it("renders hours, rounds, the round reset and what expired (en/fr)", async () => {
    expect(entryText(entry("time_passed", payload), await makeT("en"))).toBe(
      "Time passed: 1.5 h (90 rd), turn 4 → 0 — expired: Bless",
    );
    expect(entryText(entry("time_passed", payload), await makeT("fr"))).toBe(
      "Temps écoulé : 1,5 h (90 rd.), tour 4 → 0 — expirés : Bless",
    );
  });
  it("without expiries", async () => {
    expect(
      entryText(
        entry("time_passed", {
          ...payload,
          hours: 8,
          rounds: 480,
          expired: undefined,
        }),
        await makeT("en"),
      ),
    ).toBe("Time passed: 8 h (480 rd), turn 4 → 0");
  });
});

describe("inventory entries", () => {
  const use = {
    itemId: "wand_lightning",
    name: "Wand of lightning",
    type: "wand",
    behaviour: "charges",
    remaining: 0,
    maxCount: 4,
    dice: "6d6",
    rolls: [1, 2, 3, 4, 5, 6],
    total: 21,
    save: "",
    notes: "",
    breakdown: "",
  };

  it("renders a use that removed the item", async () => {
    const t = await makeT("en");
    expect(entryText(entry("item_use", { ...use, removed: true }), t)).toBe(
      "Used Wand of lightning: 6d6 = 21 (last charge, removed)",
    );
    expect(
      entryText(entry("item_use", { ...use, dice: "", removed: true }), t),
    ).toBe("Used Wand of lightning (last charge, removed)");
  });

  it("renders a use that kept the item, and old entries without behaviour", async () => {
    const t = await makeT("en");
    expect(
      entryText(entry("item_use", { ...use, removed: false, remaining: 3 }), t),
    ).toBe("Used Wand of lightning: 6d6 = 21 (3 left)");
    const old = {
      itemId: "gem",
      name: "Gem",
      kind: "charged",
      remaining: 0,
      maxCount: 1,
      dice: "",
      rolls: [],
      total: 0,
    };
    expect(entryText(entry("item_use", old), t)).toBe("Used Gem (0 left)");
  });

  it("renders a restock in both languages", async () => {
    const p = {
      itemId: "arrows_plus3",
      name: "Arrows +3",
      type: "arrow",
      added: 1,
      count: 1,
    };
    expect(entryText(entry("item_restock", p), await makeT("en"))).toBe(
      "Restocked Arrows +3: +1 (1 now)",
    );
    expect(entryText(entry("item_restock", p), await makeT("fr"))).toBe(
      "Arrows +3 réapprovisionné : +1 (1 maintenant)",
    );
  });

  it("falls back to the server summary for an unknown restock payload", async () => {
    expect(
      entryText(entry("item_restock", { name: "x" }, "srv"), await makeT("en")),
    ).toBe("srv");
  });

  it("renders item add, edit and remove entries in both languages", async () => {
    const base = { itemId: "w", name: "Wand of lightning", count: 3 };
    const edit = entry("item_edit", {
      ...base,
      changes: [{ field: "count", before: "4", after: "3" }],
    });
    expect(entryText(edit, await makeT("en"))).toBe(
      "Edited Wand of lightning: count 4 → 3",
    );
    expect(entryText(edit, await makeT("fr"))).toBe(
      "Wand of lightning modifié : quantité 4 → 3",
    );
    expect(entryText(entry("item_add", base), await makeT("en"))).toBe(
      "Added Wand of lightning (3)",
    );
    expect(entryText(entry("item_remove", base), await makeT("en"))).toBe(
      "Removed Wand of lightning (3)",
    );
    expect(
      entryText(entry("item_remove", { name: "x" }, "srv"), await makeT("en")),
    ).toBe("srv");
  });

  it("renders weapon add, edit and remove entries in both languages", async () => {
    const base = { weapon_id: "bastard_sword", name: "Bastard Sword +2" };
    const edit = entry("weapon_edit", {
      ...base,
      changes: [
        { field: "magic_to_hit", before: "2", after: "3" },
        { field: "magic_damage", before: "2", after: "3" },
      ],
    });
    expect(entryText(edit, await makeT("en"))).toBe(
      "Edited weapon Bastard Sword +2: to-hit bonus 2 → 3, damage bonus 2 → 3",
    );
    expect(entryText(edit, await makeT("fr"))).toBe(
      "Arme Bastard Sword +2 modifiée : bonus au toucher 2 → 3, bonus aux dégâts 2 → 3",
    );
    expect(entryText(entry("weapon_add", base), await makeT("en"))).toBe(
      "Added weapon Bastard Sword +2",
    );
    expect(entryText(entry("weapon_remove", base), await makeT("en"))).toBe(
      "Removed weapon Bastard Sword +2",
    );
    expect(
      entryText(
        entry("weapon_remove", { weapon_id: "x" }, "srv"),
        await makeT("en"),
      ),
    ).toBe("srv");
  });

  it("renders XP add, edit and remove entries in both languages", async () => {
    const award = (base: number, bonus: boolean, reason: string) => ({
      id: 3,
      date: "2026-10-15",
      session: "",
      reason,
      base,
      bonus,
      bonusAmount: bonus ? Math.round(base / 10) : 0,
      total: bonus ? base + Math.round(base / 10) : base,
    });
    const troll = award(1205, true, "troll");
    const en = await makeT("en");
    const fr = await makeT("fr");
    expect(entryText(entry("xp_add", { entry: troll }), en)).toBe(
      "XP +1,326: troll",
    );
    expect(
      entryText(entry("xp_add", { entry: award(400, false, "") }), fr),
    ).toBe("XP +400 : XP gagnés");
    expect(entryText(entry("xp_remove", { entry: troll }), en)).toBe(
      "XP removed: troll (1,326 XP)",
    );
    const edit = {
      entry: award(500, false, "ogre"),
      before: award(400, false, "troll"),
    };
    expect(entryText(entry("xp_edit", edit), en)).toBe(
      "XP edited: ogre, 400 → 500 XP",
    );
    expect(entryText(entry("xp_edit", edit), fr)).toBe(
      "XP modifiés : ogre, 400 → 500 XP",
    );
    const renamed = { entry: award(400, false, "ogre"), before: edit.before };
    expect(entryText(entry("xp_edit", renamed), en)).toBe(
      "XP edited: ogre (400 XP)",
    );
    expect(entryText(entry("xp_remove", {}, "srv"), en)).toBe("srv");
  });
});

describe("cast entries", () => {
  const cast = (extra: object) =>
    entry("cast", {
      action: "cast",
      spellId: "x_spell",
      spell: "X Spell",
      casterLevel: 15,
      ...extra,
    });

  it("names the mode of a received Meteor Swarm and of a default Wall of Ice", async () => {
    const t = await makeT("en");
    const swarm = entry("cast", {
      action: "received",
      spellId: "meteor_swarm",
      spell: "Meteor Swarm",
      casterLevel: 18,
      mode: "small_spheres",
      damage: { total: 50, type: "fire", dice: "5d4", instances: 8 },
    });
    expect(entryText(swarm, t)).toBe(
      "Meteor Swarm cast on me (caster level 18): 8 small spheres, 50 fire damage",
    );
    const wall = entry("cast", {
      action: "cast",
      spellId: "wall_of_ice",
      spell: "Wall of Ice",
      casterLevel: 15,
      mode: "ice_plane",
    });
    expect(entryText(wall, t)).toBe("Cast Wall of Ice: Ice plane");
  });

  it("a cast logged before modes existed renders as before, no option named", async () => {
    const old = entry("cast", {
      action: "cast",
      spellId: "meteor_swarm",
      spell: "Meteor Swarm",
      casterLevel: 15,
      damage: { total: 99, type: "fire" },
    });
    expect(entryText(old, await makeT("en"))).toBe(
      "Cast Meteor Swarm: 99 fire damage",
    );
  });

  it("names the cast-time mode (en/fr); an unknown key reads as its words", async () => {
    const e = entry("cast", {
      action: "cast",
      spellId: "wall_of_ice",
      spell: "Wall of Ice",
      casterLevel: 15,
      mode: "ice_sheet",
      damage: { total: 19, type: "cold" },
    });
    expect(entryText(e, await makeT("en"))).toBe(
      "Cast Wall of Ice: Ice sheet, 19 cold damage",
    );
    expect(entryText(e, await makeT("fr"))).toContain("Plaque de glace");
    expect(entryText(cast({ mode: "some_mode" }), await makeT("en"))).toBe(
      "Cast X Spell: some mode",
    );
  });

  it("renders the extra slots a cast granted (en/fr)", async () => {
    const e = cast({ extraSlots: [1, 1, 1] });
    expect(entryText(e, await makeT("en"))).toBe(
      "Cast X Spell: extra slots 3 × level 1",
    );
    expect(entryText(cast({ extraSlots: [3] }), await makeT("en"))).toBe(
      "Cast X Spell: extra slots level 3",
    );
    expect(entryText(cast({ extraSlots: [1, 2] }), await makeT("fr"))).toBe(
      "Lancé : X Spell: emplacements supplémentaires niveau 1 + niveau 2",
    );
  });

  it("renders the item spent and the to-hit roll (en/fr)", async () => {
    const e = cast({
      item: { itemId: "ruby", name: "Ruby", remaining: 1 },
      attack: { d20: 14, targetAc: 4, hit: true },
    });
    expect(entryText(e, await makeT("en"))).toBe(
      "Cast X Spell: Ruby spent (1 left), hit (d20 14 vs AC 4)",
    );
    expect(
      entryText(
        cast({ attack: { d20: 3, targetAc: 4, hit: false } }),
        await makeT("fr"),
      ),
    ).toBe("Lancé : X Spell: raté (d20 3 contre CA 4)");
  });
});

describe("spell_prep entries", () => {
  it("lists the spells with their copies (en/fr)", async () => {
    const e = entry("spell_prep", {
      spells: [
        { id: "magic_missile", name: "Magic Missile", count: 1 },
        { id: "mirror_image", name: "Mirror Image", count: 2 },
      ],
    });
    expect(entryText(e, await makeT("en"))).toBe(
      "Prepared: Magic Missile, Mirror Image ×2",
    );
    expect(entryText(e, await makeT("fr"))).toMatch(/^Préparé : .+ ×2$/);
    expect(
      entryText(entry("spell_prep", { spells: [] }), await makeT("en")),
    ).toBe("Prepared no spells");
  });

  it("falls back to the summary of an entry logged before the payload", async () => {
    expect(
      entryText(entry("spell_prep", {}, "Prepared: Alarm"), await makeT("en")),
    ).toBe("Prepared: Alarm");
  });
});

// Entries logged before the payloads carried every field (English summary, "label", "source",
// "damageBreakdown", "breakdown", the cast's "orb") still render, worded from what they have.
describe("entries logged before the change", () => {
  it("an old save, attack, item use and orb cast render as before", async () => {
    const t = await makeT("en");
    const save = entry(
      "save",
      {
        category: "spell",
        label: "Spells",
        d20Roll: 12,
        target: 14,
        total: 13,
        passed: false,
        margin: -1,
        rolledPassed: false,
        adjustment: 0,
        modifiers: [{ kind: "buff", source: "Luck", detail: "Luck", value: 1 }],
      },
      "Save vs Spells: d20 12 vs 14, FAILED",
    );
    expect(entryText(save, t)).toBe("Save vs Spells: d20 12 vs 14, FAILED");
    const attack = entry(
      "attack",
      {
        hit: true,
        isCrit: false,
        weaponName: "Long Sword +2",
        targetAc: 5,
        totalDamage: 7,
        damageBreakdown: "((5 × 1.0) + 2) × 1.0 + 0 = 7",
        modifiers: [
          { kind: "strength", source: "Strength 12", detail: "12", value: 0 },
        ],
      },
      "Long Sword +2 vs AC 5: hit, 7 damage",
    );
    expect(entryText(attack, t)).toBe("Long Sword +2 vs AC 5: hit, 7 damage");
    const use = entry(
      "item_use",
      {
        name: "Wand of lightning",
        dice: "6d6",
        total: 21,
        remaining: 3,
        rolls: [1, 2, 3, 4, 5, 6],
        breakdown: "6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21",
      },
      "Used Wand of lightning: 6d6 = 21 (3 left)",
    );
    expect(entryText(use, t)).toBe("Used Wand of lightning: 6d6 = 21 (3 left)");
    const orb = entry(
      "cast",
      {
        action: "cast",
        spellId: "chromatic_orb",
        spell: "Chromatic Orb",
        casterLevel: 15,
        orb: { gem: "Diamond", d20: 20, targetAc: 4, hit: true },
      },
      "Cast Chromatic Orb: Diamond, hit (d20 20 vs AC 4)",
    );
    expect(entryText(orb, t)).toBe("Cast Chromatic Orb");
  });
});

describe("initiative entries", () => {
  it("words the d10 and its round in English and French", async () => {
    const e = entry("initiative", { round: 3, d10: 7 }, "");
    expect(entryText(e, await makeT("en"))).toBe(
      "Initiative 7 (d10) for turn 3",
    );
    expect(entryText(e, await makeT("fr"))).toBe(
      "Initiative 7 (d10) pour le tour 3",
    );
  });

  it("falls back to the summary without a payload", async () => {
    expect(entryText(entry("initiative", null, "old"), await makeT("en"))).toBe(
      "old",
    );
  });
});

describe("entryText for skill checks", () => {
  const prof = {
    id: "riding_land",
    kind: "proficiency",
    die: 20,
    roll: 11,
    target: 11,
    success: true,
    ability: "wis",
    abilityScore: 8,
    modifier: 3,
  };
  const bard = {
    id: "pick_pockets",
    kind: "bardSkill",
    die: 100,
    roll: 51,
    target: 50,
    success: false,
  };

  it("words the check from its payload (en, fr), never the server summary", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    expect(entryText(entry("skill_check", prof, ""), en)).toBe(
      "Riding, land-based: d20 11 vs 11, success",
    );
    expect(entryText(entry("skill_check", bard, ""), en)).toBe(
      "Pick pockets: d100 51 vs 50, failure",
    );
    expect(entryText(entry("skill_check", bard, ""), fr)).toBe(
      "Vol à la tire : d100 51 contre 50, échec",
    );
  });

  it("falls back to the summary on an unknown payload", async () => {
    expect(
      entryText(entry("skill_check", { id: "x" }), await makeT("en")),
    ).toBe("server summary");
  });

  it("shows the ability as DEX 17 −1, leaving out a 0 modifier", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    expect(abilityText(en, "dex", 17, -1)).toBe("DEX 17 −1");
    expect(abilityText(en, "wis", 8, 3)).toBe("WIS 8 +3");
    expect(abilityText(fr, "wis", 8, 0)).toBe("SAG 8");
  });
});

describe("entryText for dice rolls", () => {
  const roll = {
    formula: "2d6+3",
    groups: [{ sides: 6, rolls: [2, 5], negative: false, subtotal: 7 }],
    modifier: 3,
    total: 10,
  };
  it("words the formula and the total (en, fr), an unreadable payload falls back", async () => {
    expect(entryText(entry("dice", roll, ""), await makeT("en"))).toBe(
      "Dice 2d6+3 = 10",
    );
    expect(entryText(entry("dice", roll, ""), await makeT("fr"))).toBe(
      "Dés 2d6+3 = 10",
    );
    expect(entryText(entry("dice", { formula: "x" }), await makeT("en"))).toBe(
      "server summary",
    );
  });
});

describe("hp_change entries without a source", () => {
  const hp = (delta: number, summary: string): LogEntry => ({
    ...entry("hp_change", {}, summary),
    hpDelta: delta,
    effectiveHpDelta: delta,
  });

  it.each([
    ["en", -5, "Damage 5"],
    ["en", 4, "Healing 4"],
    ["fr", -5, "Dégâts 5"],
    ["fr", 4, "Soins 4"],
  ])("%s, delta %i: %s", async (lng, delta, want) => {
    expect(entryText(hp(delta, ""), await makeT(lng))).toBe(want);
  });

  it("keeps the player's own text when there is one", async () => {
    expect(entryText(hp(-5, "orc axe"), await makeT("en"))).toBe("orc axe");
  });
});
