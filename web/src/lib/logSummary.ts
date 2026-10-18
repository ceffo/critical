import type { TFunction } from "i18next";
import type { LogEntry, LogKind } from "@/types/log";
import type { Modifier, RollResult, WeaponChangeRecord } from "@/types/combat";
import type { AbilityCheck, SkillCheck } from "@/types/skills";
import type { DiceRoll } from "@/types/dice";
import type { XpChangeRecord } from "@/types/xp";
import type { LearnResult, LevelUpRecord, TurnResult } from "@/types/character";
import { type InventoryNamer, recordedNames } from "@/i18n/inventoryName";
import { buffName, englishSpellName, type SpellNamer } from "@/i18n/spellName";
import { checkAdjustedTarget } from "./skillAdjust";
import { formatSigned } from "@/lib/format";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import type { SpellExposure, SpellFailure } from "@/types/spells";

/** Payload of cast entries (castsvc castRecord). */
export interface CastLogPayload {
  action: "cast" | "received" | "effect";
  spellId: string;
  spell: string;
  casterLevel: number;
  /** The key of the cast-time mode; absent for a spell without modes and on older casts. */
  mode?: string;
  buff?: {
    name: string;
    rounds?: number;
    charges?: number;
    strGain?: number;
    applied: boolean;
  };
  damage?: {
    total: number;
    type: string;
    /** Absent on casts logged before the dice were recorded. */
    dice?: string;
    instances?: number;
    rounds?: number;
    /** Every instance roll, round by round; `parts` are its part rolls (damage made of parts). */
    rolls?: {
      formula: string;
      dice: { sides: number; value: number }[];
      total: number;
      parts?: {
        formula: string;
        dice: { sides: number; value: number }[];
        total: number;
      }[];
      /** The round or instance of a damage made of dice steps; absent without steps and on older casts. */
      step?: number;
    }[];
    /** The parts of one instance and what a save does to each; absent without parts and on older casts. */
    parts?: { formula: string; save: string }[];
    /** The total without the parts a save negates; absent without parts and on older casts. */
    totalOnSave?: number;
    /** "round" or "instance" for a damage made of dice steps; absent otherwise and on older casts. */
    stepsPer?: "round" | "instance";
  };
  /** Levels of the extra slots the cast granted. */
  extraSlots?: number[];
  /** The inventory item the cast spent. */
  item?: { itemId: string; name: string; remaining: number };
  /** The spell's to-hit roll. */
  attack?: { d20: number; targetAc: number; hit: boolean };
  /** The target's saving throw (the DM's side); absent on older casts. */
  save?: { type: string; effect: string };
  /** The exposure damage served at casterLevel (as GET /api/spells serves it); absent without one. */
  exposure?: SpellExposure;
  /** The low-Wisdom spell failure d100 (ADR 0030 step 14); absent: no die. */
  failure?: SpellFailure;
}

/** Payload of exposure entries (castsvc exposureRecord, ADR 0008 step 16). */
export interface ExposureLogPayload {
  spellId: string;
  spell: string;
  casterLevel: number;
  /** The exposure mode's key; absent without modes. */
  mode?: string;
  round: number;
  /** The Bard was exposed: the entry's HP delta is the damage. */
  onSelf: boolean;
  damage: NonNullable<CastLogPayload["damage"]>;
  /** The exposure damage served at casterLevel, for the next round. */
  exposure?: SpellExposure;
  /** Set by the server when the HP floor cut an exposure on the Bard: what was asked and applied. */
  hpRequested?: number;
  hpApplied?: number;
}

export function exposurePayload(e: LogEntry): ExposureLogPayload | null {
  if (e.kind !== "exposure") return null;
  const p = e.payload as Partial<ExposureLogPayload> | null;
  return p &&
    typeof p.spellId === "string" &&
    typeof p.round === "number" &&
    p.damage
    ? (p as ExposureLogPayload)
    : null;
}

/** "Cloudkill exposure, round 2 (Within 10 ft): 7 fire damage, on me". */
function exposureText(
  p: ExposureLogPayload,
  t: TFunction,
  spellName: SpellNamer,
): string {
  const spell = spellName(p.spellId, p.spell);
  const text = t(p.onSelf ? "log.exposure.onMe" : "log.exposure.forDm", {
    spell,
    round: p.round,
    total: p.damage.total,
    type: p.damage.type
      ? t(`spellDice.types.${p.damage.type}`, { defaultValue: p.damage.type })
      : "",
  });
  const parts = [text];
  if (p.mode) parts.push(modeName(t, p.spellId, p.mode));
  // Cut by the HP floor (ADR 0011): the server merged what was asked and applied into the payload.
  if (
    typeof p.hpRequested === "number" &&
    typeof p.hpApplied === "number" &&
    p.hpRequested !== p.hpApplied
  )
    parts.push(t("log.floored", { requested: formatSigned(p.hpRequested) }));
  return parts.join(" · ");
}

export function castPayload(e: LogEntry): CastLogPayload | null {
  if (e.kind !== "cast") return null;
  const p = e.payload as Partial<CastLogPayload> | null;
  return p && typeof p.action === "string" && typeof p.spellId === "string"
    ? (p as CastLogPayload)
    : null;
}

function castText(
  p: CastLogPayload,
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): string {
  const spell = spellName(p.spellId, p.spell);
  const head = t(`log.cast.${p.action}`, { spell, level: p.casterLevel });
  const details: string[] = [];
  // ADR 0030 step 14: the spell failure d100 first; a failed spell has nothing else.
  if (p.failure)
    details.push(
      t(p.failure.failed ? "log.cast.spellFailed" : "log.cast.spellHeld", {
        roll: p.failure.d100,
        chance: p.failure.chance,
      }),
    );
  if (p.mode) details.push(modeName(t, p.spellId, p.mode));
  if (p.buff?.applied) {
    details.push(counters(t, p.buff.rounds, p.buff.charges));
    if (p.buff.strGain)
      details.push(t("log.cast.strGain", { gain: p.buff.strGain }));
  }
  if (p.damage)
    details.push(
      t("log.cast.damage", { total: p.damage.total, type: p.damage.type }),
    );
  if (p.extraSlots && p.extraSlots.length > 0)
    details.push(
      t("log.cast.extraSlots", { slots: splitText(t, p.extraSlots) }),
    );
  if (p.item)
    details.push(
      t("log.cast.item", {
        name: names.item(p.item.itemId, p.item.name),
        left: p.item.remaining,
      }),
    );
  if (p.attack)
    details.push(
      t(p.attack.hit ? "log.cast.attackHit" : "log.cast.attackMiss", {
        d20: p.attack.d20,
        ac: p.attack.targetAc,
      }),
    );
  return details.length > 0 ? `${head}: ${details.join(", ")}` : head;
}
import type { ItemChangeRecord, RestockResult, UseResult } from "@/types/items";

/** Payload of long_rest entries (restsvc restRecord). */
function restText(e: LogEntry, t: TFunction): string | null {
  if (e.kind !== "long_rest") return null;
  const p = e.payload as {
    slotsReset?: number;
    itemsRefilled?: string[];
  } | null;
  if (!p || typeof p.slotsReset !== "number") return null;
  return t("log.rest", {
    slots: p.slotsReset,
    items: p.itemsRefilled?.length ?? 0,
  });
}

/** Payload of initiative entries (buffsvc initiativePayload): the round and its d10. */
function initiativeText(e: LogEntry, t: TFunction): string | null {
  if (e.kind !== "initiative") return null;
  const p = e.payload as { round?: number; d10?: number } | null;
  if (!p || typeof p.d10 !== "number" || typeof p.round !== "number")
    return null;
  return t("log.initiative", { value: p.d10, round: p.round });
}

/** Payload of spell_prep entries (prepsvc prepRecord): each spell once, with its copies. */
function prepText(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer,
): string | null {
  if (e.kind !== "spell_prep") return null;
  const p = e.payload as {
    spells?: { id: string; name: string; count: number }[];
  } | null;
  if (!p || !Array.isArray(p.spells)) return null;
  if (p.spells.length === 0) return t("log.prepNone");
  const spells = p.spells
    .map((s) => {
      const name = spellName(s.id, s.name);
      return s.count > 1 ? `${name} ×${s.count}` : name;
    })
    .join(", ");
  return t("log.prep", { spells });
}

/** Payload of buff_change / round_* / time_passed entries (buffsvc logPayload). */
export interface BuffLogPayload {
  action:
    | "activate"
    | "edit"
    | "absorb"
    | "dismiss"
    | "next_round"
    | "end_combat"
    | "set_round"
    | "pass_time"
    | "save_template"
    | "delete_template";
  /** pass_time: the hours typed (rounds = the rounds that passed). */
  hours?: number;
  buff?: string;
  /** The buff's spell when it comes from one (catalogue id); `buff` is the stored name. */
  spellId?: string;
  rounds?: number;
  charges?: number;
  fromRounds?: number;
  fromCharges?: number;
  ended?: boolean;
  fromRound?: number;
  toRound?: number;
  expired?: string[];
  /** Parallel to `expired`: each expired buff's spell, "" when not from one; absent when none is. */
  expiredSpellIds?: string[];
  note?: string;
  fromNote?: string;
  noteChanged?: boolean;
}

function buffPayload(e: LogEntry): BuffLogPayload | null {
  if (
    e.kind !== "buff_change" &&
    e.kind !== "round_advance" &&
    e.kind !== "round_reset" &&
    e.kind !== "round_set" &&
    e.kind !== "time_passed"
  ) {
    return null;
  }
  const p = e.payload as Partial<BuffLogPayload> | null;
  return p && typeof p.action === "string" ? (p as BuffLogPayload) : null;
}

/** "5 rd, 3 ch", or "until dismissed" when the buff has neither counter. */
export function counters(t: TFunction, rounds = 0, charges = 0): string {
  const parts = [
    rounds > 0 ? t("buffs.rounds", { count: rounds }) : null,
    charges > 0 ? t("buffs.charges", { count: charges }) : null,
  ].filter((p): p is string => p !== null);
  return parts.length > 0 ? parts.join(", ") : t("buffs.untilDismissed");
}

/**
 * Payload of attack entries: the roll breakdown (combatsvc attackRecord, same shape as RollResult); a
 * corrected attack's is the latest amend's.
 */
export function attackPayload(e: LogEntry): RollResult | null {
  if (e.kind !== "attack") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<RollResult> | null;
  return p && typeof p.hit === "boolean" && typeof p.weaponName === "string"
    ? (p as RollResult)
    : null;
}

/** Payload of save entries (combatsvc saveRecord); an adjusted save's is the latest amend's. */
export interface SaveLogPayload {
  category: string;
  d20Roll: number;
  target: number;
  passed: boolean;
  adjustment: number;
  adjustmentReason?: string;
  /** Verdict before the latest adjustment; set once the save was adjusted. */
  previousPassed?: boolean;
  /** The modifiers applied when rolling (buffs), not the adjustment. */
  modifiers?: Modifier[];
  /** rollTotal + adjustment (kept by the server; not displayed). */
  total?: number;
  /** d20 + modifiers, the adjustment left out (ADR 0006 step 5). */
  rollTotal: number;
  /** target − adjustment: the adjustment moves the target, + easier, − harder. */
  adjustedTarget: number;
  /** rollTotal − adjustedTarget, as computed by the server. */
  margin: number;
}

export function savePayload(e: LogEntry): SaveLogPayload | null {
  if (e.kind !== "save") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<SaveLogPayload> | null;
  return p &&
    typeof p.passed === "boolean" &&
    typeof p.d20Roll === "number" &&
    typeof p.rollTotal === "number" &&
    typeof p.adjustedTarget === "number" &&
    typeof p.margin === "number" &&
    typeof p.category === "string"
    ? (p as SaveLogPayload)
    : null;
}

/** The roll side of a save as the journal words it: "d20 12", or "d20 12 +2 = 14" with buffs. */
function saveRollText(p: SaveLogPayload): string {
  return p.rollTotal === p.d20Roll
    ? `d20 ${p.d20Roll}`
    : `d20 ${p.d20Roll} ${formatSigned(p.rollTotal - p.d20Roll)} = ${p.rollTotal}`;
}

function saveText(p: SaveLogPayload, t: TFunction): string {
  const verdict = (passed: boolean) =>
    t(passed ? "result.passed" : "result.failed").toUpperCase();
  const args = {
    name: t(`saves.categories.${p.category}`, { defaultValue: p.category }),
    roll: saveRollText(p),
    target: p.target,
  };
  if (p.previousPassed === undefined)
    return t("log.saveThrow.rolled", { ...args, verdict: verdict(p.passed) });
  const change = {
    ...args,
    from: verdict(p.previousPassed),
    to: verdict(p.passed),
  };
  if (p.adjustment === 0) return t("log.saveThrow.removed", change);
  return t("log.saveThrow.adjusted", {
    ...change,
    adjustedTarget: p.adjustedTarget,
    adjustment: formatSigned(p.adjustment),
    reason: p.adjustmentReason ? ` ${p.adjustmentReason}` : "",
  });
}

/** Payload of item_use entries (itemsvc useRecord, same shape as UseResult). */
export function itemPayload(e: LogEntry): UseResult | null {
  if (e.kind !== "item_use") return null;
  const p = e.payload as Partial<UseResult> | null;
  return p && typeof p.name === "string" && typeof p.remaining === "number"
    ? (p as UseResult)
    : null;
}

function itemText(p: UseResult, t: TFunction, names: InventoryNamer): string {
  const args = {
    name: names.item(p.itemId, p.name),
    dice: p.dice,
    total: p.total,
    left: p.remaining,
  };
  if (p.removed)
    return t(p.dice ? "log.item.rolledRemoved" : "log.item.usedRemoved", args);
  return t(p.dice ? "log.item.rolled" : "log.item.used", args);
}

/** Payload of item_restock entries (itemsvc restockRecord, same shape as RestockResult). */
function restockText(
  e: LogEntry,
  t: TFunction,
  names: InventoryNamer,
): string | null {
  if (e.kind !== "item_restock") return null;
  const p = e.payload as Partial<RestockResult> | null;
  if (
    !p ||
    typeof p.name !== "string" ||
    typeof p.added !== "number" ||
    typeof p.count !== "number"
  )
    return null;
  return t("log.item.restocked", {
    name: names.item(p.itemId, p.name),
    added: p.added,
    count: p.count,
  });
}

/** Payload of item_add, item_edit and item_remove entries (itemsvc changeRecord). */
function itemChangeText(
  e: LogEntry,
  t: TFunction,
  names: InventoryNamer,
): string | null {
  if (
    e.kind !== "item_add" &&
    e.kind !== "item_edit" &&
    e.kind !== "item_remove"
  )
    return null;
  const p = e.payload as Partial<ItemChangeRecord> | null;
  if (!p || typeof p.name !== "string" || typeof p.count !== "number")
    return null;
  const name = names.item(p.itemId, p.name);
  if (e.kind === "item_add")
    return t("log.item.added", { name, count: p.count });
  if (e.kind === "item_remove")
    return t("log.item.removedItem", { name, count: p.count });
  const changes = (p.changes ?? [])
    .map((c) =>
      t("log.item.change", {
        ...c,
        field: t(`log.item.fields.${c.field}`, { defaultValue: c.field }),
      }),
    )
    .join(", ");
  return t("log.item.edited", { name, changes });
}

/** Payload of weapon_add, weapon_edit and weapon_remove entries (weaponsvc changeRecord). */
function weaponChangeText(
  e: LogEntry,
  t: TFunction,
  names: InventoryNamer,
): string | null {
  if (
    e.kind !== "weapon_add" &&
    e.kind !== "weapon_edit" &&
    e.kind !== "weapon_remove"
  )
    return null;
  const p = e.payload as Partial<WeaponChangeRecord> | null;
  if (!p || typeof p.name !== "string") return null;
  const name = names.weapon(p.weapon_id, p.name);
  if (e.kind === "weapon_add") return t("log.weapon.added", { name });
  if (e.kind === "weapon_remove") return t("log.weapon.removed", { name });
  const changes = (p.changes ?? [])
    .map((c) =>
      t("log.weapon.change", {
        ...c,
        field: t(`log.weapon.fields.${c.field}`, { defaultValue: c.field }),
      }),
    )
    .join(", ");
  return t("log.weapon.edited", { name, changes });
}

/** Payload of xp_add, xp_edit and xp_remove entries (xpsvc changeRecord, ADR 0015 step 4). */
function xpChangeText(e: LogEntry, t: TFunction): string | null {
  if (e.kind !== "xp_add" && e.kind !== "xp_edit" && e.kind !== "xp_remove")
    return null;
  const p = e.payload as Partial<XpChangeRecord> | null;
  const now = p?.entry;
  if (!now || typeof now.total !== "number") return null;
  const reason = now.reason || t("xp.defaultReason");
  if (e.kind === "xp_add")
    return t("log.xp.added", { total: now.total, reason });
  if (e.kind === "xp_remove")
    return t("log.xp.removed", { total: now.total, reason });
  const was = p.before;
  if (!was || was.total === now.total)
    return t("log.xp.editedSame", { total: now.total, reason });
  return t("log.xp.edited", { from: was.total, to: now.total, reason });
}

/** The payload of a spell_learn entry (learnsvc record, ADR 0030 step 20.2), or null. */
export function learnPayload(e: LogEntry): LearnResult | null {
  if (e.kind !== "spell_learn") return null;
  const p = e.payload as Partial<LearnResult> | null;
  return p && typeof p.spellId === "string" && typeof p.roll === "number"
    ? (p as LearnResult)
    : null;
}

/** A chance-to-learn roll: the spell, the d100 against the chance, learned or failed. */
function learnText(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer,
): string | null {
  const p = learnPayload(e);
  if (!p) return null;
  const vars = { spell: spellName(p.spellId), roll: p.roll, chance: p.chance };
  return p.learned
    ? t("log.learn.learned", vars)
    : t("log.learn.failed", { ...vars, level: p.retryLevel });
}

/** The payload of a turn_undead entry (turnsvc record, ADR 0030 step 16), or null. */
export function turnPayload(e: LogEntry): TurnResult | null {
  if (e.kind !== "turn_undead") return null;
  const p = e.payload as Partial<TurnResult> | null;
  return p && typeof p.undead === "string" && typeof p.success === "boolean"
    ? (p as TurnResult)
    : null;
}

/** The undead type's name (PHB Table 61 row code). */
export function undeadName(t: TFunction, code: string): string {
  return t(`turning.undead.${code}`, { defaultValue: code });
}

/** "Turning undead (Ghoul): 7 turned, d20 12 vs 10": the type, the result and the d20 if rolled. */
function turnText(e: LogEntry, t: TFunction): string | null {
  const p = turnPayload(e);
  if (!p) return null;
  const undead = undeadName(t, p.undead);
  const result = !p.success
    ? t("log.turn.failed")
    : p.destroyed > 0 && p.turned > 0
      ? t("log.turn.destroyedTurned", {
          destroyed: p.destroyed,
          turned: p.turned,
        })
      : p.destroyed > 0
        ? t("log.turn.destroyed", { count: p.destroyed })
        : t("log.turn.turned", { count: p.turned });
  const roll =
    p.d20 !== undefined && p.target !== undefined
      ? t("log.turn.roll", { d20: p.d20, target: p.target })
      : t("log.turn.noRoll");
  return t("log.turn.text", { undead, result, roll });
}

/** The payload of a level_up entry (levelupsvc record, ADR 0030 step 7.3), or null. */
export function levelUpPayload(e: LogEntry): LevelUpRecord | null {
  if (e.kind !== "level_up") return null;
  const p = e.payload as Partial<LevelUpRecord> | null;
  return p && typeof p.gain === "number" && typeof p.level === "number"
    ? (p as LevelUpRecord)
    : null;
}

/** A level-up roll: the die (and the value it counts as from Constitution 20), the Constitution
 * adjustment and the gain the server computed, or the fixed gain above the name level. */
function levelUpText(e: LogEntry, t: TFunction): string | null {
  const p = levelUpPayload(e);
  if (!p) return null;
  const hp = {
    level: p.level,
    gain: p.gain,
    from: p.hpMaxBefore,
    to: p.hpMaxAfter,
  };
  if (p.fixed || p.rolled === null) return t("log.levelUp.fixed", hp);
  const die =
    p.counted !== null && p.counted !== p.rolled
      ? t("log.levelUp.dieRaised", {
          die: p.hitDie,
          rolled: p.rolled,
          counted: p.counted,
        })
      : t("log.levelUp.die", { die: p.hitDie, rolled: p.rolled });
  const con = p.conAdjustment
    ? t("log.levelUp.con", { adjustment: formatSigned(p.conAdjustment) })
    : "";
  const manual = p.manual ? t("log.levelUp.manual") : "";
  return t("log.levelUp.rolled", { ...hp, die, con, manual });
}

function attackText(
  p: RollResult,
  t: TFunction,
  names: InventoryNamer,
): string {
  const weapon = attackWeapon(p, t, names);
  // Against an unknown AC (docs/adr/0012): the AC it hits until the GM settles it, then the GM's
  // verdict (no crit).
  if (p.targetAc === null) {
    const hits = p.hitsAc ?? p.targetACNeeded;
    if (p.verdict === undefined)
      return t("log.attack.unknown", { weapon, hits });
    return p.verdict === "hit"
      ? t("log.attack.verdictHit", { weapon, damage: p.totalDamage })
      : t("log.attack.verdictMiss", { weapon });
  }
  const args = {
    weapon,
    ac: p.targetAc,
    damage: p.totalDamage,
    mult: p.critMultiplier,
    check: p.d100CritRoll,
    roll: p.d100MultiplierRoll,
  };
  // Attacks logged before the multiplier d100 existed keep the line without the rolls.
  const crit =
    p.d100MultiplierRoll === undefined
      ? "log.attack.crit"
      : "log.attack.critRolls";
  const text = !p.hit
    ? t("log.attack.miss", args)
    : t(p.isCrit ? crit : "log.attack.hit", args);
  if (p.rolledTargetAc === undefined || p.rolledHit === undefined) return text;
  const outcome = (hit: boolean, crit: boolean | undefined) =>
    t(`log.attack.outcome.${crit ? "crit" : hit ? "hit" : "miss"}`);
  return t("log.attack.corrected", {
    text,
    from: p.rolledTargetAc,
    to: p.targetAc,
    was: outcome(p.rolledHit, p.rolledCrit),
    now: outcome(p.hit, p.isCrit),
  });
}

/** Payload of skill_check entries (skillsvc checkRecord, ADR 0014): the check as rolled. */
export function skillCheckPayload(e: LogEntry): SkillCheck | null {
  if (e.kind !== "skill_check") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<SkillCheck> | null;
  return p &&
    typeof p.id === "string" &&
    typeof p.roll === "number" &&
    typeof p.target === "number" &&
    typeof p.success === "boolean" &&
    (p.kind === "proficiency" ||
      p.kind === "bardSkill" ||
      p.kind === "thiefSkill")
    ? (p as SkillCheck)
    : null;
}

/** Payload in effect of ability_check entries (skillsvc abilityRecord, ADR 0028 step 5). */
export function abilityCheckPayload(e: LogEntry): AbilityCheck | null {
  if (e.kind !== "ability_check") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<AbilityCheck> | null;
  return p &&
    typeof p.ability === "string" &&
    typeof p.roll === "number" &&
    typeof p.target === "number" &&
    typeof p.adjustedTarget === "number" &&
    typeof p.success === "boolean"
    ? (p as AbilityCheck)
    : null;
}

/** Payload of dice entries (dicesvc rollRecord): the free roll as recorded. */
export function dicePayload(e: LogEntry): DiceRoll | null {
  if (e.kind !== "dice") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<DiceRoll> | null;
  return p &&
    typeof p.formula === "string" &&
    typeof p.total === "number" &&
    typeof p.modifier === "number" &&
    Array.isArray(p.groups)
    ? (p as DiceRoll)
    : null;
}

/** Payload of correction entries (actionlog.CorrectionStep, ADR 0010 step 6): a correction made
 * after a roll, its own step on the action stack. from/to are the field's value before and after
 * (null: an unknown AC, no verdict). */
export interface CorrectionLogPayload {
  entryId: number;
  entryKind: LogKind;
  field:
    | "targetAc"
    | "saveAdjustment"
    | "skillAdjustment"
    | "abilityAdjustment"
    | "verdict"
    | string;
  from: number | string | null;
  to: number | string | null;
}

export function correctionPayload(e: LogEntry): CorrectionLogPayload | null {
  if (e.kind !== "correction") return null;
  const p = e.payload as Partial<CorrectionLogPayload> | null;
  return p && typeof p.entryId === "number" && typeof p.field === "string"
    ? (p as CorrectionLogPayload)
    : null;
}

function correctionValue(
  field: string,
  v: number | string | null,
  t: TFunction,
): string {
  switch (field) {
    case "targetAc":
      return typeof v === "number" ? String(v) : t("log.correction.unknown");
    case "saveAdjustment":
    case "skillAdjustment":
    case "abilityAdjustment":
      return typeof v === "number" ? formatSigned(v) : t("log.correction.none");
    case "verdict":
      return v === "hit" || v === "miss"
        ? t(`result.${v}`)
        : t("log.correction.none");
    default:
      return v === null ? t("log.correction.none") : String(v);
  }
}

function correctionText(p: CorrectionLogPayload, t: TFunction): string {
  if (
    p.field !== "targetAc" &&
    p.field !== "saveAdjustment" &&
    p.field !== "skillAdjustment" &&
    p.field !== "abilityAdjustment" &&
    p.field !== "verdict"
  )
    return t("log.correction.other", { id: p.entryId });
  return t(`log.correction.${p.field}`, {
    from: correctionValue(p.field, p.from, t),
    to: correctionValue(p.field, p.to, t),
  });
}

/** A skill's display name; an id without a translation shows as is. */
export function skillName(t: TFunction, id: string): string {
  return t(`skills.names.${id}`, { defaultValue: id });
}

/** "DEX 17 −1" (the modifier left out when 0). */
export function abilityText(
  t: TFunction,
  ability: string,
  score: number,
  modifier: number,
): string {
  const name = t(`attr.${ability}`, { defaultValue: ability });
  return modifier === 0
    ? `${name} ${score}`
    : `${name} ${score} ${formatSigned(modifier)}`;
}

function skillText(c: SkillCheck, t: TFunction): string {
  if (c.adjustment) {
    return t(
      c.success ? "log.skill.successAdjusted" : "log.skill.failureAdjusted",
      {
        name: skillName(t, c.id),
        die: c.die,
        roll: c.roll,
        target: c.target,
        adjustedTarget: checkAdjustedTarget(c),
        adjustment: formatSigned(c.adjustment),
        reason: c.adjustmentReason ? ` ${c.adjustmentReason}` : "",
      },
    );
  }
  return t(c.success ? "log.skill.success" : "log.skill.failure", {
    name: skillName(t, c.id),
    die: c.die,
    roll: c.roll,
    target: c.target,
  });
}

/**
 * "DEX: d20 14 vs 13 (15 −2 ledge), failure": the ability check as the journal words it (ADR 0028
 * step 5), its adjustment and reason when set. The verdict is the payload's.
 */
function abilityCheckText(c: AbilityCheck, t: TFunction): string {
  const name = t(`attr.${c.ability}`, { defaultValue: c.ability });
  if (c.adjustment) {
    return t(
      c.success
        ? "abilityCheck.log.successAdjusted"
        : "abilityCheck.log.failureAdjusted",
      {
        name,
        die: c.die,
        roll: c.roll,
        target: c.target,
        adjustedTarget: c.adjustedTarget,
        adjustment: formatSigned(c.adjustment),
        reason: c.adjustmentReason ? ` ${c.adjustmentReason}` : "",
      },
    );
  }
  return t(
    c.success ? "abilityCheck.log.success" : "abilityCheck.log.failure",
    { name, die: c.die, roll: c.roll, target: c.target },
  );
}

/** An HP change logged without a source has no player text: word the kind from its delta. */
function hpChangeText(e: LogEntry, t: TFunction): string | null {
  if (e.kind !== "hp_change" || e.effectiveSummary) return null;
  return t(e.hpDelta < 0 ? "log.hpDamage" : "log.hpHeal", {
    amount: Math.abs(e.hpDelta),
  });
}

/**
 * An attack's weapon, with its ammunition when it fired one: "Short Bow +2 with Arrows +1". Both are
 * named by id in the player's language while the inventory has them (ADR 0023 step 12).
 */
export function attackWeapon(
  p: Pick<RollResult, "weaponId" | "weaponName" | "ammo">,
  t: TFunction,
  names: InventoryNamer = recordedNames,
): string {
  const weapon = names.weapon(p.weaponId, p.weaponName ?? "");
  return p.ammo
    ? t("log.attack.withAmmo", {
        weapon,
        ammo: names.item(p.ammo.itemId, p.ammo.name),
      })
    : weapon;
}

/**
 * The entry's text in the player's language, worded from its structured payload. The server's summary
 * is only the player's own text (an HP change's source) or, on entries logged before the payloads
 * existed, an older English line. Spell names come from `spellName` (useSpellName in a component),
 * item and weapon names from `names` (useInventoryName).
 */
export function entryText(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): string {
  const attack = attackPayload(e);
  if (attack) return attackText(attack, t, names);
  const skill = skillCheckPayload(e);
  if (skill) return skillText(skill, t);
  const ability = abilityCheckPayload(e);
  if (ability) return abilityCheckText(ability, t);
  const dice = dicePayload(e);
  if (dice) return t("log.dice", { formula: dice.formula, total: dice.total });
  const save = savePayload(e);
  if (save) return saveText(save, t);
  const correction = correctionPayload(e);
  if (correction) return correctionText(correction, t);
  const cast = castPayload(e);
  if (cast) return castText(cast, t, spellName, names);
  const exposure = exposurePayload(e);
  if (exposure) return exposureText(exposure, t, spellName);
  const rest = restText(e, t);
  if (rest) return rest;
  const prep = prepText(e, t, spellName);
  if (prep) return prep;
  const initiative = initiativeText(e, t);
  if (initiative) return initiative;
  const item = itemPayload(e);
  if (item) return itemText(item, t, names);
  const restock = restockText(e, t, names);
  if (restock) return restock;
  const itemChange = itemChangeText(e, t, names);
  if (itemChange) return itemChange;
  const weaponChange = weaponChangeText(e, t, names);
  if (weaponChange) return weaponChange;
  const xpChange = xpChangeText(e, t);
  if (xpChange) return xpChange;
  const levelUp = levelUpText(e, t);
  if (levelUp) return levelUp;
  const learn = learnText(e, t, spellName);
  if (learn) return learn;
  const turn = turnText(e, t);
  if (turn) return turn;
  const hp = hpChangeText(e, t);
  if (hp) return hp;
  const p = buffPayload(e);
  if (!p) return e.effectiveSummary;
  const buff = buffName(spellName, { name: p.buff ?? "", spellId: p.spellId });
  const expired = (p.expired ?? []).map((name, i) =>
    buffName(spellName, { name, spellId: p.expiredSpellIds?.[i] }),
  );
  switch (p.action) {
    case "activate":
      return t("log.buff.activate", {
        buff,
        counters: counters(t, p.rounds, p.charges),
      });
    case "edit": {
      const sameCounters =
        (p.fromRounds ?? 0) === (p.rounds ?? 0) &&
        (p.fromCharges ?? 0) === (p.charges ?? 0);
      if (p.noteChanged && !p.ended && sameCounters) {
        return p.note
          ? t("log.buff.editNote", { buff, note: p.note })
          : t("log.buff.editNoteCleared", { buff });
      }
      const text = t(p.ended ? "log.buff.editEnded" : "log.buff.edit", {
        buff,
        from: counters(t, p.fromRounds, p.fromCharges),
        to: counters(t, p.rounds, p.charges),
      });
      return p.noteChanged && !p.ended
        ? `${text} ${t("log.buff.noteAlso")}`
        : text;
    }
    case "absorb":
      return p.ended
        ? t("log.buff.absorbEnded", { buff })
        : t("log.buff.absorb", { buff, count: p.charges ?? 0 });
    case "save_template":
      return t("log.buff.saveTemplate", {
        buff,
        counters: counters(t, p.rounds, p.charges),
      });
    case "delete_template":
      return t("log.buff.deleteTemplate", { buff });
    case "dismiss":
      return t("log.buff.dismiss", { buff });
    case "next_round": {
      const text = t("log.buff.nextRound", {
        from: p.fromRound ?? 0,
        to: p.toRound ?? 0,
      });
      return expired.length > 0
        ? `${text} ${t("log.buff.expired", { names: expired.join(", ") })}`
        : text;
    }
    case "pass_time": {
      const text = t("log.buff.passTime", {
        hours: p.hours ?? 0,
        rounds: t("buffs.rounds", { count: p.rounds ?? 0 }),
        from: p.fromRound ?? 0,
        to: p.toRound ?? 0,
      });
      return expired.length > 0
        ? `${text} ${t("log.buff.expired", { names: expired.join(", ") })}`
        : text;
    }
    case "end_combat":
      return t("log.buff.endCombat", {
        from: p.fromRound ?? 0,
        to: p.toRound ?? 0,
      });
    case "set_round":
      return t("log.buff.setRound", {
        from: p.fromRound ?? 0,
        to: p.toRound ?? 0,
      });
    default:
      return e.effectiveSummary;
  }
}
