import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { LogEntry } from "@/types/log";
import { entryText } from "./entry";
import { abilityText } from "./words";
import { spellNamer } from "@/i18n/spellName";
import { inventoryNamer } from "@/i18n/inventoryName";

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
      "Saved effect Hammer form (10 rd)",
    ],
    [
      { action: "delete_template", buff: "Hammer form" },
      "Deleted saved effect Hammer form",
    ],
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

  it("translates template and note log texts to French", async () => {
    const t = await makeT("fr");
    const text = (p: object) => entryText(entry("buff_change", p), t);
    expect(text({ action: "save_template", buff: "Marteau", rounds: 10 })).toBe(
      "Effet Marteau enregistré (10 tr.)",
    );
    expect(text({ action: "delete_template", buff: "Marteau" })).toBe(
      "Effet enregistré Marteau supprimé",
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
      "Long Sword +2 contre CA 5 : critique (jet 7 · multiplicateur 63 → ×1,5), 10 dégâts",
    ],
  ])("renders attack %j in %s", async (payload, lng, want) => {
    const t = await makeT(lng);
    expect(entryText(entry("attack", payload, "server"), t)).toBe(want);
  });

  // ADR 0037 step 18: a proficiencies entry names the languages gained and lost, in the player's
  // language, from the language catalogue (the code when it has none); no change, no language line.
  it("names the languages a proficiencies entry gains and loses", async () => {
    const catalogue = [
      { code: "elvish", name: { en: "Elvish", fr: "Elfique" } },
      { code: "dwarvish", name: { en: "Dwarvish", fr: "Nain" } },
    ];
    const picks = [{ id: "tumbling", slots: 1 }];
    const withLanguages = (before: string[], after: string[]) =>
      entry("proficiencies", {
        before: { nonweapon: picks, languages: before },
        after: { nonweapon: picks, languages: after },
      });
    const text = async (lng: string, before: string[], after: string[]) =>
      entryText(
        withLanguages(before, after),
        await makeT(lng),
        undefined,
        inventoryNamer([], [], lng, catalogue),
      );
    expect(await text("fr", ["common", "dwarvish"], ["common", "elvish"])).toBe(
      "Langues : + Elfique, − Nain",
    );
    expect(await text("en", ["common"], ["common", "elvish", "gnome"])).toBe(
      "Languages: + Elvish, + gnome",
    );
    expect(
      await text("fr", ["common", "elvish"], ["elvish", "common"]),
    ).not.toContain("Langues");
  });

  // ADR 0023 step 12: the weapon, its ammunition and a used item are named by id in the player's
  // language while the inventory has them; a removed one keeps its recorded name.
  it("names the weapon, the ammunition and the item from the inventory", async () => {
    const t = await makeT("fr");
    const names = inventoryNamer(
      [
        { id: "arrows_plus1", name: { en: "Arrows +1", fr: "Flèches +1" } },
        {
          id: "power_gem_dispel",
          name: { en: "Power Gem", fr: "Gemme de pouvoir" },
        },
      ],
      [{ id: "shortbow", name: { en: "Short Bow +2", fr: "Arc court +2" } }],
      "fr",
    );
    const shot = {
      hit: false,
      totalDamage: 0,
      weaponId: "shortbow",
      weaponName: "Short Bow +2",
      targetAc: 5,
      ammo: {
        itemId: "arrows_plus1",
        name: "Arrows +1",
        magic: 1,
        remaining: 3,
      },
    };
    expect(
      entryText(entry("attack", shot, "server"), t, undefined, names),
    ).toBe("Arc court +2 (Flèches +1) contre CA 5 : attaque ratée");
    const use = {
      itemId: "power_gem_dispel",
      name: "Power Gem",
      remaining: 0,
      rolls: [],
      total: 0,
    };
    expect(entryText(entry("item_use", use), t, undefined, names)).toBe(
      "Objet utilisé : Gemme de pouvoir (0 restant)",
    );
    expect(
      entryText(
        entry("item_use", { ...use, itemId: "gone" }),
        t,
        undefined,
        names,
      ),
    ).toBe("Objet utilisé : Power Gem (0 restant)");
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
      "Long Sword +2 contre CA 10 : critique (jet 3 · multiplicateur 77 → ×1,5), 10 dégâts (CA -10 → 10 : RATÉ → CRITIQUE)",
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
      "Long Sword +2 vs unknown AC: hit (DM), 7 damage",
    ],
    [
      "fr",
      { verdict: "miss" },
      "Long Sword +2 contre CA inconnue : attaque ratée (MJ)",
    ],
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
    rollTotal: 12,
    adjustedTarget: 14,
    margin: -2,
  };

  // The adjustment moves the target (ADR 0006 step 5): the roll side is the d20 and the buffs.
  it.each([
    ["rolled", rolled, "Save vs Spells: d20 12 vs 14, FAILED"],
    [
      "rolled with a buff",
      {
        ...rolled,
        modifiers: [{ kind: "buff", detail: "Luck", value: 1 }],
        rollTotal: 13,
        margin: -1,
      },
      "Save vs Spells: d20 12 +1 = 13 vs 14, FAILED",
    ],
    [
      "adjusted with a reason",
      {
        ...rolled,
        adjustment: 4,
        adjustmentReason: "Gaia fire",
        adjustedTarget: 10,
        passed: true,
        margin: 2,
        previousPassed: false,
      },
      "Save vs Spells: d20 12 vs 10 (target 14, adjusted +4 Gaia fire): FAILED → PASSED",
    ],
    [
      "adjusted without a reason: a −2 raises the target",
      {
        ...rolled,
        d20Roll: 15,
        rollTotal: 15,
        adjustment: -2,
        adjustedTarget: 16,
        passed: false,
        previousPassed: true,
      },
      "Save vs Spells: d20 15 vs 16 (target 14, adjusted −2): PASSED → FAILED",
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
        adjustedTarget: 10,
        passed: true,
        previousPassed: false,
      },
    };
    expect(entryText(e, t)).toBe(
      "Save vs Spells: d20 12 vs 10 (target 14, adjusted +4): FAILED → PASSED",
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
          adjustedTarget: 10,
          passed: true,
          previousPassed: false,
        }),
        t,
      ),
    ).toBe(
      "Sauvegarde contre Sorts : d20 12 contre 10 (seuil 14, ajusté +4 Gaia feu) : ÉCHOUÉE → RÉUSSIE",
    );
  });

  it("renders a removed adjustment in French", async () => {
    const t = await makeT("fr");
    expect(
      entryText(entry("save", { ...rolled, previousPassed: true }), t),
    ).toBe(
      "Sauvegarde contre Sorts : d20 12 contre 14, ajustement retiré : RÉUSSIE → ÉCHOUÉE",
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
      "Time passed: 1.5 h (90 rd), round 4 → 0 — expired: Bless",
    );
    expect(entryText(entry("time_passed", payload), await makeT("fr"))).toBe(
      "Temps écoulé : 1,5 h (90 tr.), tour 4 → 0 — expirés : Bless",
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
    ).toBe("Time passed: 8 h (480 rd), round 4 → 0");
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
      "Objet réapprovisionné : Arrows +3, +1 (1 maintenant)",
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
      "Objet modifié : Wand of lightning, quantité 4 → 3",
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

  it("words a feminine or masculine item name without a past participle on it", async () => {
    const fr = await makeT("fr");
    const base = { itemId: "p", name: "Potion de soins", count: 2 };
    expect(entryText(entry("item_add", base), fr)).toBe(
      "Objet ajouté : Potion de soins (2)",
    );
    expect(entryText(entry("item_remove", base), fr)).toBe(
      "Objet retiré : Potion de soins (2)",
    );
    expect(
      entryText(
        entry("item_use", { name: "Potion de soins", remaining: 1 }),
        fr,
      ),
    ).toBe("Objet utilisé : Potion de soins (1 restant)");
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

  // ADR 0033 step 7 worded with the one term of ADR 0035 item 3: équipé / retiré, a swap names
  // the piece taken off.
  it("renders an equipment state change in both languages", async () => {
    const move = {
      itemId: "chain",
      name: "Chain mail",
      category: "armour",
      before: "carried",
      after: "worn",
    };
    const en = await makeT("en");
    const fr = await makeT("fr");
    expect(entryText(entry("equipment_state", move), en)).toBe(
      "Equipped: Chain mail",
    );
    expect(entryText(entry("equipment_state", move), fr)).toBe(
      "Équipé : Chain mail",
    );
    const putAway = { ...move, before: "worn", after: "carried" };
    expect(entryText(entry("equipment_state", putAway), fr)).toBe(
      "Retiré : Chain mail",
    );
    const swap = {
      ...move,
      off: [
        {
          ...move,
          itemId: "leather",
          name: "Leather",
          before: "worn",
          after: "carried",
        },
      ],
    };
    expect(entryText(entry("equipment_state", swap), en)).toBe(
      "Equipped: Chain mail (in place of Leather)",
    );
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

// ADR 0030 step 7.3: a level_up entry is structured (the level, the die and its raised value, the
// Constitution adjustment, the gain and the max HP around it); the journal words it, French first.
describe("level-up entries", () => {
  const roll = (over: object) =>
    entry("level_up", {
      level: 9,
      hitDie: 6,
      rolled: 4,
      counted: 4,
      conAdjustment: 2,
      gain: 6,
      fixed: false,
      manual: false,
      hpMaxBefore: 40,
      hpMaxAfter: 46,
      ...over,
    });

  it("words the die, the CON adjustment and the gain", async () => {
    const fr = await makeT("fr");
    const en = await makeT("en");
    expect(entryText(roll({}), fr)).toBe(
      "Niveau 9 : d6 4, CON +2, PV max +6 (40 → 46)",
    );
    expect(entryText(roll({}), en)).toBe(
      "Level 9: d6 4, CON +2, max HP +6 (40 → 46)",
    );
  });

  it("shows a low die raised by Constitution 20+, no CON term at 0, a die entered", async () => {
    const fr = await makeT("fr");
    const en = await makeT("en");
    expect(
      entryText(
        roll({
          rolled: 1,
          counted: 2,
          conAdjustment: 0,
          gain: 2,
          hpMaxAfter: 42,
          manual: true,
        }),
        fr,
      ),
    ).toBe("Niveau 9 : d6 1 (compté 2), PV max +2 (40 → 42), dé saisi");
    expect(
      entryText(
        roll({
          rolled: 1,
          counted: 2,
          conAdjustment: -1,
          gain: 1,
          hpMaxAfter: 41,
        }),
        en,
      ),
    ).toBe("Level 9: d6 1 (counts as 2), CON −1, max HP +1 (40 → 41)");
  });

  it("words a fixed gain above the name level", async () => {
    const fr = await makeT("fr");
    const en = await makeT("en");
    const fixed = roll({
      level: 16,
      hitDie: 0,
      rolled: null,
      counted: null,
      conAdjustment: 0,
      gain: 2,
      fixed: true,
      hpMaxBefore: 85,
      hpMaxAfter: 87,
    });
    expect(entryText(fixed, fr)).toBe(
      "Niveau 16 : PV max +2, gain fixe (85 → 87)",
    );
    expect(entryText(fixed, en)).toBe(
      "Level 16: max HP +2, fixed gain (85 → 87)",
    );
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

  it("words a received spell with a gender-neutral label in French", async () => {
    const fr = await makeT("fr");
    expect(
      entryText(
        entry("cast", {
          action: "received",
          spellId: "mirror_image",
          spell: "Image miroir",
          casterLevel: 9,
        }),
        fr,
      ),
    ).toMatch(/^Sort reçu : Image miroir \(niveau du lanceur 9\)/);
    expect(
      fr("result.cast.receivedHeading", {
        name: "Image miroir",
        casterLevel: 9,
      }),
    ).toBe("Sort reçu : Image miroir, niveau du lanceur 9");
    expect(
      fr("result.cast.castHeading", {
        name: "Image miroir",
        level: 2,
        casterLevel: 9,
      }),
    ).toBe("Image miroir (niveau 2), niveau du lanceur 9");
  });

  it("words a French weapon attack and a dismissed effect without a participle on the name", async () => {
    const fr = await makeT("fr");
    expect(
      entryText(
        entry("buff_change", { action: "dismiss", buff: "Image miroir" }),
        fr,
      ),
    ).toBe("Effet retiré : Image miroir");
    expect(fr("log.attack.miss", { weapon: "Dague", ac: 4 })).toBe(
      "Dague contre CA 4 : attaque ratée",
    );
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
      "Meteor Swarm received (caster level 18): 8 small spheres, 50 fire damage",
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
    ).toBe("Lancé : X Spell: attaque ratée (d20 3 contre CA 4)");
  });
});

// The catalogue's names (GET /api/spells/names): Mirror Image has no French name here.
const frNames = spellNamer("fr", {
  magic_missile: {
    id: "magic_missile",
    name: { en: "Magic Missile", fr: "Projectile magique" },
  },
  mirror_image: { id: "mirror_image", name: { en: "Mirror Image" } },
});

describe("cast entries name the spell from the catalogue", () => {
  it("in French, falling back to the payload's English name", async () => {
    const cast = (spellId: string, spell: string) =>
      entry("cast", { action: "cast", spellId, spell, casterLevel: 5 });
    const fr = await makeT("fr");
    expect(
      entryText(cast("magic_missile", "Magic Missile"), fr, frNames),
    ).toContain("Projectile magique");
    expect(entryText(cast("armor", "Armor"), fr, frNames)).toContain("Armor");
    expect(
      entryText(cast("magic_missile", "Magic Missile"), await makeT("en")),
    ).toBe("Cast Magic Missile");
  });
});

// Buff entries of a buff from a spell carry its id (spellId, expiredSpellIds parallel to expired):
// named from the catalogue; a buff not from a spell, and an entry recorded before, keep the name.
describe("buff entries name a spell's buff from the catalogue", () => {
  const frBuffNames = spellNamer("fr", {
    stoneskin: {
      id: "stoneskin",
      name: { en: "Stoneskin", fr: "Peau de pierre" },
    },
    haste: { id: "haste", name: { en: "Haste", fr: "Hâte" } },
  });

  it("dismiss, absorb and edit in French; the stored name without a spell id", async () => {
    const fr = await makeT("fr");
    const text = (p: object) =>
      entryText(entry("buff_change", p), fr, frBuffNames);
    expect(
      text({ action: "dismiss", buff: "Stoneskin", spellId: "stoneskin" }),
    ).toContain("Peau de pierre");
    expect(
      text({
        action: "absorb",
        buff: "Stoneskin",
        spellId: "stoneskin",
        charges: 2,
      }),
    ).toContain("Peau de pierre");
    expect(
      text({
        action: "edit",
        buff: "Haste",
        spellId: "haste",
        fromRounds: 4,
        rounds: 2,
      }),
    ).toContain("Hâte");
    const old = text({ action: "dismiss", buff: "Stoneskin" });
    expect(old).toContain("Stoneskin");
    expect(old).not.toContain("Peau de pierre");
  });

  it("names the expired buffs in order, each by its own spell id", async () => {
    const fr = await makeT("fr");
    const got = entryText(
      entry("round_advance", {
        action: "next_round",
        fromRound: 4,
        toRound: 5,
        expired: ["Haste", "Rage", "Stoneskin"],
        expiredSpellIds: ["haste", "", "stoneskin"],
      }),
      fr,
      frBuffNames,
    );
    expect(got).toContain("Hâte, Rage, Peau de pierre");
    const passTime = entryText(
      entry("time_passed", {
        action: "pass_time",
        hours: 1,
        rounds: 60,
        fromRound: 0,
        toRound: 0,
        expired: ["Haste"],
      }),
      fr,
      frBuffNames,
    );
    expect(passTime).toContain("Haste");
    expect(passTime).not.toContain("Hâte");
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
    expect(entryText(e, await makeT("fr"), frNames)).toBe(
      "Préparé : Projectile magique, Mirror Image ×2",
    );
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
        // Filled by the server on read (ADR 0006 step 8).
        rollTotal: 13,
        adjustedTarget: 14,
      },
      "Save vs Spells: d20 12 vs 14, FAILED",
    );
    expect(entryText(save, t)).toBe(
      "Save vs Spells: d20 12 +1 = 13 vs 14, FAILED",
    );
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
      "Initiative 7 (d10) for round 3",
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

describe("entryText for ability checks", () => {
  const dex = {
    ability: "dex",
    die: 20,
    roll: 14,
    abilityScore: 15,
    target: 15,
    adjustment: 0,
    adjustedTarget: 15,
    success: true,
    rolledSuccess: true,
  };

  it("words the check from its payload (en, fr)", async () => {
    expect(entryText(entry("ability_check", dex, ""), await makeT("en"))).toBe(
      "DEX check: d20 14 vs 15, success",
    );
    expect(entryText(entry("ability_check", dex, ""), await makeT("fr"))).toBe(
      "Jet de DEX : d20 14 contre 15, réussi",
    );
  });

  it("words an adjusted check with its reason; the verdict is the payload's (en, fr)", async () => {
    const harder = {
      ...dex,
      adjustment: -2,
      adjustmentReason: "ledge",
      adjustedTarget: 13,
      success: false,
    };
    expect(
      entryText(entry("ability_check", harder, ""), await makeT("en")),
    ).toBe("DEX check: d20 14 vs 13 (15 −2 ledge), failure");
    expect(
      entryText(
        entry("ability_check", { ...harder, adjustmentReason: undefined }, ""),
        await makeT("fr"),
      ),
    ).toBe("Jet de DEX : d20 14 contre 13 (15 −2), échec");
  });

  it("words the adjustment's correction step (en, fr)", async () => {
    const step = {
      entryId: 7,
      entryKind: "ability_check",
      field: "abilityAdjustment",
      from: 0,
      to: -2,
    };
    expect(entryText(entry("correction", step, ""), await makeT("en"))).toBe(
      "Correction ability check adjustment 0 → −2",
    );
    expect(entryText(entry("correction", step, ""), await makeT("fr"))).toBe(
      "Correction ajustement de jet de caractéristique 0 → −2",
    );
  });

  it("falls back to the summary on an unknown payload", async () => {
    expect(
      entryText(entry("ability_check", { ability: "dex" }), await makeT("en")),
    ).toBe("server summary");
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

  it("words a thief skill check (ADR 0030 step 17) like a bard skill's (fr)", async () => {
    const fr = await makeT("fr");
    const thief = {
      ...bard,
      id: "open_locks",
      kind: "thiefSkill",
      roll: 40,
      target: 55,
      success: true,
    };
    expect(entryText(entry("skill_check", thief, ""), fr)).toBe(
      "Crochetage : d100 40 contre 55, réussi",
    );
    const en = await makeT("en");
    expect(entryText(entry("skill_check", thief, ""), en)).toMatch(
      /^Open Locks: d100 40 vs 55/,
    );
  });

  it("words a thief's skill points spent (ADR 0030 step 17): each changed skill (en, fr)", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    const e = entry(
      "skill_points",
      {
        before: { open_locks: 30, pick_pockets: 30 },
        after: { open_locks: 45, pick_pockets: 30, climb_walls: 15 },
      },
      "",
    );
    expect(entryText(e, en)).toBe(
      "Skill points: Open Locks 30 → 45, Climb walls 0 → 15",
    );
    expect(entryText(e, fr)).toBe(
      "Points de talent : Crochetage 30 → 45, Escalade 0 → 15",
    );
  });

  it("words a thief's skill armour change (ADR 0030 step 1): before and after (en, fr)", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    const e = entry(
      "skill_armor",
      { before: "none", after: "elven_chain" },
      "",
    );
    expect(entryText(e, en)).toBe("Skill armour: None → Elven chain");
    expect(entryText(e, fr)).toBe(
      "Armure des talents : Aucune → Cotte de mailles elfique",
    );
  });

  it("words the AC override set and cleared (ADR 0033 step 13) (en, fr)", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    const set = entry(
      "ac_override",
      { before: null, after: { value: -1, label: "Shield spell" } },
      "",
    );
    expect(entryText(set, en)).toBe("AC overridden: −1 (Shield spell)");
    expect(entryText(set, fr)).toBe("CA forcée : −1 (Shield spell)");
    const bare = entry(
      "ac_override",
      { before: null, after: { value: 4 } },
      "",
    );
    expect(entryText(bare, fr)).toBe("CA forcée : 4");
    const cleared = entry(
      "ac_override",
      { before: { value: 4 }, after: null },
      "",
    );
    expect(entryText(cleared, en)).toBe("AC override cleared");
    expect(entryText(cleared, fr)).toBe("CA forcée effacée");
  });

  it("words an adjusted check: adjusted target, original target, adjustment and reason (en, fr)", async () => {
    const en = await makeT("en");
    const fr = await makeT("fr");
    const adjusted = {
      ...prof,
      roll: 12,
      target: 11,
      adjustment: -2,
      adjustmentReason: "in the dark",
      adjustedTarget: 9,
      success: false,
      rolledSuccess: false,
    };
    expect(entryText(entry("skill_check", adjusted, ""), en)).toBe(
      "Riding, land-based: d20 12 vs 9 (11 −2 in the dark), failure",
    );
    expect(
      entryText(
        entry(
          "skill_check",
          {
            ...adjusted,
            reason: undefined,
            adjustmentReason: undefined,
            adjustment: 5,
            adjustedTarget: 16,
            success: true,
          },
          "",
        ),
        fr,
      ),
    ).toBe("Équitation, à terre : d20 12 contre 16 (11 +5), réussi");
    // Adjustment removed after the roll: worded as a plain check again.
    expect(
      entryText(
        entry(
          "skill_check",
          { ...prof, adjustment: 0, adjustedTarget: 11 },
          "",
        ),
        en,
      ),
    ).toBe("Riding, land-based: d20 11 vs 11, success");
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

// ADR 0030 step 20.2: a spell_learn entry is structured (the spell, the d100, the chance, learned
// or not, the realized level); the journal words it, French first (the test namer falls back to
// the id).
describe("spell learn entries", () => {
  const roll = (over: object) =>
    entry("spell_learn", {
      spellId: "sleep",
      spellLevel: 1,
      roll: 37,
      chance: 65,
      learned: true,
      characterLevel: 3,
      retryLevel: 0,
      ...over,
    });

  it("words a success and a failure with the level to try again at", async () => {
    const fr = await makeT("fr");
    const en = await makeT("en");
    expect(entryText(roll({}), fr)).toBe("Appris : sleep (d100 37 ≤ 65 %)");
    expect(
      entryText(roll({ roll: 72, learned: false, retryLevel: 4 }), fr),
    ).toBe("Échec : sleep (d100 72 > 65 %), à retenter au niveau 4");
    expect(entryText(roll({}), en)).toBe("Learned: sleep (d100 37 ≤ 65%)");
    // The level comes from the server's retryLevel, never worked out here.
    expect(entryText(roll({ learned: false, retryLevel: 6 }), fr)).toMatch(
      /au niveau 6$/,
    );
  });
});

// ADR 0030 step 16: a turn_undead entry is structured (the type, the cell, the dice, the destroyed and
// turned counts the server worked out); the journal words it, French first.
describe("turn undead entries", () => {
  const turn = (over: object) =>
    entry("turn_undead", {
      undead: "ghoul",
      clericLevel: 3,
      target: 10,
      d20: 12,
      success: true,
      effect: "turn",
      affected: [3, 4],
      destroyed: 0,
      turned: 7,
      ...over,
    });

  it("words a rolled turn, a failure and a destroy without a roll", async () => {
    const fr = await makeT("fr");
    const en = await makeT("en");
    expect(entryText(turn({}), fr)).toBe(
      "Renvoi des morts-vivants (Goule) : 7 repoussés, d20 12 contre 10",
    );
    expect(
      entryText(
        turn({ d20: 4, success: false, effect: undefined, turned: 0 }),
        fr,
      ),
    ).toBe("Renvoi des morts-vivants (Goule) : échec, d20 4 contre 10");
    expect(
      entryText(
        turn({
          undead: "skeleton",
          target: undefined,
          d20: undefined,
          outcome: "destroy_more",
          effect: "destroy",
          more: [1, 2],
          destroyed: 7,
          turned: 3,
        }),
        en,
      ),
    ).toBe("Turning undead (Skeleton): 7 destroyed, 3 turned, no roll");
    expect(
      entryText(
        turn({
          target: undefined,
          d20: undefined,
          outcome: "destroy",
          effect: "destroy",
          destroyed: 7,
          turned: 0,
        }),
        fr,
      ),
    ).toBe("Renvoi des morts-vivants (Goule) : 7 détruits, sans jet");
  });
});

// ADR 0030 step 14: a cast with a spell failure d100 says it, failed or not.
describe("spell failure on a cast", () => {
  const cast = (failed: boolean) =>
    entry("cast", {
      action: "cast",
      spellId: "bless",
      spell: "Bless",
      casterLevel: 3,
      failure: { d100: failed ? 12 : 55, chance: 30, failed },
    });

  it("words a failed spell and a spell that held", async () => {
    const fr = await makeT("fr");
    expect(entryText(cast(true), fr)).toBe(
      "Lancé : Bless: échec du sort (d100 12 ≤ 30 %)",
    );
    expect(entryText(cast(false), fr)).toBe(
      "Lancé : Bless: pas d'échec du sort (d100 55 > 30 %)",
    );
  });
});

// ADR 0004 step 21: the entry of a row that ended with end effects words the cause, each effect's
// dice and total, and the HP before and after, in the player's language; the spell named from its id.
describe("end effects", () => {
  const names = spellNamer("fr", {
    polymorph_self: {
      id: "polymorph_self",
      name: { en: "Polymorph Self", fr: "Métamorphose" },
    },
  });
  const heal = (cause: string, value: number, constant = 0) => ({
    spellId: "polymorph_self",
    buff: "Polymorph Self",
    cause,
    kind: "heal",
    dice: constant ? `1d12+${constant}` : "1d12",
    rolls: [{ sides: 12, value }],
    constant,
    total: value + constant,
  });
  const dismiss = (cause: string, effects?: object[]) =>
    entry("buff_change", {
      action: "dismiss",
      buff: "Polymorph Self",
      spellId: "polymorph_self",
      cause,
      ...(effects ? { endEffects: effects, hpBefore: 3, hpAfter: 7 } : {}),
    });

  it("ended by me: the roll and the HP, in English and French", async () => {
    expect(
      entryText(
        dismiss("voluntary", [heal("voluntary", 4)]),
        await makeT("en"),
      ),
    ).toBe("Ended Polymorph Self: regains 4 HP (1d12: 4), HP 3 → 7");
    expect(
      entryText(
        dismiss("voluntary", [heal("voluntary", 4)]),
        await makeT("fr"),
        names,
      ),
    ).toBe("Métamorphose : fin par moi, regagne 4 PV (1d12 : 4), PV 3 → 7");
  });

  it("dispelled or slain: the cause, no heal", async () => {
    expect(entryText(dismiss("dispelled"), await makeT("en"))).toBe(
      "Polymorph Self dispelled or slain",
    );
    expect(entryText(dismiss("dispelled"), await makeT("fr"), names)).toBe(
      "Métamorphose : dissipation ou mort",
    );
  });

  it("running out on Next Round: the expired names, then the heal; a constant is shown", async () => {
    const got = entryText(
      entry("round_advance", {
        action: "next_round",
        fromRound: 3,
        toRound: 4,
        expired: ["Polymorph Self"],
        expiredSpellIds: ["polymorph_self"],
        endEffects: [heal("expired", 9, 1)],
        hpBefore: 20,
        hpAfter: 30,
      }),
      await makeT("en"),
    );
    expect(got).toBe(
      "Round 3 → 4 — expired: Polymorph Self. Polymorph Self regains 10 HP (1d12+1: 9 + 1), HP 20 → 30",
    );
  });

  it("at full HP the cap gave nothing: the roll shows, not a gain", async () => {
    const full = entry("buff_change", {
      action: "dismiss",
      buff: "Polymorph Self",
      spellId: "polymorph_self",
      cause: "voluntary",
      endEffects: [heal("voluntary", 9)],
      hpBefore: 40,
      hpAfter: 40,
    });
    expect(entryText(full, await makeT("en"))).toBe(
      "Ended Polymorph Self: rolls 1d12: 9, no HP regained, HP 40 → 40",
    );
    expect(entryText(full, await makeT("fr"))).toContain(
      "jette 1d12 : 9, aucun PV regagné, PV 40 → 40",
    );
  });
});
