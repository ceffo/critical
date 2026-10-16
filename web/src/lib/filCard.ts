// The result panel and the journal (Act activity, ADR 0022): which journal entry the panel shows, what each entry kind renders from its
// payload, and the journal's filter. Display only: verdicts, totals and dice are the server's, read
// from the payloads as recorded (docs/all.md, "Core Architectural Rule").
import type { TFunction } from "i18next";
import type { HpBreakdown, LogEntry, LogKind } from "@/types/log";
import type { Modifier, RollResult } from "@/types/combat";
import type { UseResult } from "@/types/items";
import type { CastResult, ExposureResult } from "@/types/spells";
import type { SkillCheck } from "@/types/skills";
import type { DiceRoll } from "@/types/dice";
import { englishSpellName, type SpellNamer } from "@/i18n/spellName";
import { type InventoryNamer, recordedNames } from "@/i18n/inventoryName";
import { formatSigned, formatTime } from "./format";
import { attackSettled } from "./attackAdjust";
import {
  attackPayload,
  attackWeapon,
  castPayload,
  dicePayload,
  entryText,
  exposurePayload,
  type ExposureLogPayload,
  itemPayload,
  savePayload,
  skillCheckPayload,
  skillName,
  type CastLogPayload,
} from "./logSummary";

/** The last roll of this session as the roll endpoints returned it (stores/roll.ts LastRoll). */
export type LiveResult =
  | { kind: "attack"; result: RollResult }
  | { kind: "save"; category: string; result: RollResult }
  | { kind: "cast"; result: CastResult }
  | { kind: "item"; result: UseResult }
  | { kind: "skill"; result: SkillCheck }
  | { kind: "exposure"; result: ExposureResult };

/** The journal filter: everything, or rolls only. */
export type LogFilter = "all" | "rolls";

/**
 * The entries whose result is "the latest result", as the server's actionlog.RollKinds: the card
 * shows the newest of them by default, and only that one takes corrections (AC, save adjustment).
 * A free dice roll is not one: it neither replaces the card nor ends the latest save's correction
 * window; the composer shows it by selecting its entry.
 */
export const RESULT_KINDS: readonly LogKind[] = [
  "attack",
  "save",
  "cast",
  "item_use",
  "skill_check",
  "exposure",
];

/** The newest entry of RESULT_KINDS, voided ones included (the server's "latest roll"). */
export function latestResult(entries: readonly LogEntry[]): LogEntry | null {
  return entries.find((e) => RESULT_KINDS.includes(e.kind)) ?? null;
}

/**
 * The "rolls" filter: entries that carry dice the server rolled (attacks, saves, item uses,
 * initiative, skill checks, free dice rolls, and casts with a damage roll or a to-hit d20).
 */
export function isRollEntry(e: LogEntry): boolean {
  switch (e.kind) {
    case "attack":
    case "save":
    case "skill_check":
    case "dice":
    case "item_use":
    case "initiative":
    case "exposure":
      return true;
    case "cast": {
      const p = castPayload(e);
      return p !== null && (p.damage !== undefined || p.attack !== undefined);
    }
    default:
      return false;
  }
}

export function filterEntries(
  entries: LogEntry[],
  filter: LogFilter,
): LogEntry[] {
  return filter === "all" ? entries : entries.filter(isRollEntry);
}

/** The log entry recording a live result. */
export function liveEntryId(last: LiveResult): number | undefined {
  return last.kind === "cast" || last.kind === "exposure"
    ? last.result.entryId
    : last.result.logEntryId;
}

/**
 * What the detail card shows:
 * - an older entry the player selected in the journal (read-only);
 * - else the latest result: this session's last roll while it is the newest result entry (or the
 *   log has not caught up with it yet), otherwise the newest result entry from its payload;
 * - else nothing.
 * `latest` marks the latest result, the only one that offers corrections.
 */
export type CardSource =
  | { kind: "empty" }
  | { kind: "live"; last: LiveResult; entry: LogEntry | null }
  | { kind: "entry"; entry: LogEntry; latest: boolean };

export function cardSource(
  last: LiveResult | null,
  entries: readonly LogEntry[],
  selectedId: number | null,
): CardSource {
  const latest = latestResult(entries);
  const liveId = last ? liveEntryId(last) : undefined;
  if (
    selectedId !== null &&
    selectedId !== latest?.id &&
    selectedId !== liveId
  ) {
    const selected = entries.find((e) => e.id === selectedId);
    if (selected) return { kind: "entry", entry: selected, latest: false };
  }
  if (
    last &&
    (latest === null || liveId === undefined || liveId >= latest.id)
  ) {
    return {
      kind: "live",
      last,
      entry: entries.find((e) => e.id === liveId) ?? null,
    };
  }
  if (latest) return { kind: "entry", entry: latest, latest: true };
  return { kind: "empty" };
}

/** The journal row highlighted for the card: the selected entry, else the latest result's. */
export function cardEntryId(source: CardSource): number | null {
  if (source.kind === "entry") return source.entry.id;
  if (source.kind === "live") return liveEntryId(source.last) ?? null;
  return null;
}

/** A saving throw as the card shows it, from a save entry's payload or the save endpoint's result. */
export interface SaveView {
  category: string;
  d20: number;
  target: number;
  /** The modifiers applied when rolling (buffs), without the adjustment. */
  modifiers: Modifier[];
  adjustment: number;
  adjustmentReason?: string;
  /** d20 + modifiers + adjustment; absent on saves logged before it was recorded. */
  total?: number;
  passed: boolean;
  /** total − target; absent on saves logged before it was recorded. */
  margin?: number;
  logEntryId?: number;
}

/** The save endpoint lists the adjustment among the modifiers; the card shows it on its own. */
export function saveFromRoll(category: string, r: RollResult): SaveView {
  return {
    category,
    d20: r.d20Roll,
    target: r.effectiveThac0,
    modifiers: r.modifiers.filter((m) => m.kind !== "adjustment"),
    adjustment: r.adjustment ?? 0,
    adjustmentReason: r.adjustmentReason,
    total: r.saveTotal,
    passed: r.hit,
    margin: r.hitMargin,
    logEntryId: r.logEntryId,
  };
}

/** A non-roll entry: its text and, for HP changes, the delta (red for damage, green for healing). */
export interface InfoView {
  text: string;
  value: { amount: number; tone: "damage" | "heal" } | null;
  /** A change cut by max HP or the floor: what was asked for. */
  capped: HpBreakdown | null;
}

/** The card's body, one layout per entry kind; unreadable payloads fall back to info. */
export type CardModel =
  | { layout: "attack"; roll: RollResult }
  | { layout: "save"; save: SaveView }
  | { layout: "cast"; cast: CastLogPayload }
  | { layout: "castLive"; result: CastResult }
  | { layout: "exposure"; exposure: ExposureResult }
  | { layout: "item"; item: UseResult }
  | { layout: "initiative"; round: number; d10: number }
  | { layout: "skill"; check: SkillCheck }
  | { layout: "dice"; roll: DiceRoll }
  | { layout: "info"; info: InfoView };

/** HP change value as the journal and the card show it: a voided change keeps its original delta. */
function hpValue(e: LogEntry): InfoView["value"] {
  if (e.kind !== "hp_change") return null;
  const amount = e.status === "voided" ? e.hpDelta : e.effectiveHpDelta;
  return { amount, tone: amount < 0 ? "damage" : "heal" };
}

/** The server's breakdown when an HP change was cut (a heal by max HP, damage by the floor), else null. */
export function cappedHeal(e: LogEntry): HpBreakdown | null {
  if (e.kind !== "hp_change" || e.status === "voided") return null;
  const p = e.effectivePayload as Partial<HpBreakdown> | null;
  if (typeof p?.hpRequested !== "number" || typeof p.hpApplied !== "number")
    return null;
  return p.hpRequested !== p.hpApplied ? (p as HpBreakdown) : null;
}

function initiativePayload(e: LogEntry): { round: number; d10: number } | null {
  if (e.kind !== "initiative") return null;
  const p = e.payload as { round?: number; d10?: number } | null;
  return p && typeof p.round === "number" && typeof p.d10 === "number"
    ? { round: p.round, d10: p.d10 }
    : null;
}

/** The card model of a journal entry, from its (effective) payload. */
export function cardModel(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): CardModel {
  const attack = attackPayload(e);
  if (attack)
    return { layout: "attack", roll: { ...attack, logEntryId: e.id } };
  const save = savePayload(e);
  if (save) {
    return {
      layout: "save",
      save: {
        category: save.category,
        d20: save.d20Roll,
        target: save.target,
        modifiers: save.modifiers ?? [],
        adjustment: save.adjustment ?? 0,
        adjustmentReason: save.adjustmentReason,
        total: save.total,
        passed: save.passed,
        margin: save.margin,
        logEntryId: e.id,
      },
    };
  }
  const cast = castPayload(e);
  if (cast) return { layout: "cast", cast };
  const exposure = exposurePayload(e);
  if (exposure)
    return { layout: "exposure", exposure: exposureFromPayload(e, exposure) };
  const item = itemPayload(e);
  if (item) return { layout: "item", item: { ...item, logEntryId: e.id } };
  const initiative = initiativePayload(e);
  if (initiative) return { layout: "initiative", ...initiative };
  const skill = skillCheckPayload(e);
  if (skill) return { layout: "skill", check: { ...skill, logEntryId: e.id } };
  const dice = dicePayload(e);
  if (dice) return { layout: "dice", roll: { ...dice, logEntryId: e.id } };
  return {
    layout: "info",
    info: {
      text: entryText(e, t, spellName, names),
      value: hpValue(e),
      capped: cappedHeal(e),
    },
  };
}

/**
 * An exposure entry's payload as the exposure endpoint's result, so one view shows both; the HP
 * applied is the entry's (a voided one keeps its original delta, as an HP change shows it).
 */
export function exposureFromPayload(
  e: LogEntry,
  p: ExposureLogPayload,
): ExposureResult {
  return {
    entryId: e.id,
    spellId: p.spellId,
    name: p.spell,
    casterLevel: p.casterLevel,
    mode: p.mode,
    round: p.round,
    onSelf: p.onSelf,
    hpDelta: e.status === "voided" ? e.hpDelta : e.effectiveHpDelta,
    damage: {
      dice: p.damage.dice ?? "",
      type: p.damage.type,
      instances: p.damage.instances ?? 1,
      rounds: p.damage.rounds ?? 1,
      rolls: p.damage.rolls ?? [],
      total: p.damage.total,
      parts: p.damage.parts?.map((x) => ({
        formula: x.formula,
        save: x.save as "none" | "negates",
      })),
      stepsPer: p.damage.stepsPer,
    },
    exposure: p.exposure,
  };
}

/** The card model of this session's last roll. */
export function liveModel(last: LiveResult): CardModel {
  switch (last.kind) {
    case "attack":
      return { layout: "attack", roll: last.result };
    case "save":
      return { layout: "save", save: saveFromRoll(last.category, last.result) };
    case "cast":
      return { layout: "castLive", result: last.result };
    case "item":
      return { layout: "item", item: last.result };
    case "skill":
      return { layout: "skill", check: last.result };
    case "exposure":
      return { layout: "exposure", exposure: last.result };
  }
}

/** The log kind a live result is recorded as (for its tag before the log has caught up). */
export function liveKind(last: LiveResult): LogKind {
  switch (last.kind) {
    case "item":
      return "item_use";
    case "skill":
      return "skill_check";
    default:
      return last.kind;
  }
}

/** "Turn 3" or "Out of combat" (turn 0). */
export function turnLabel(t: TFunction, round: number): string {
  return round > 0 ? t("fil.turn", { round }) : t("fil.outOfCombat");
}

/** The card header: kind tag, subtitle (weapon, save, spell, item…), and "T3 · 21:31". */
export interface CardHeader {
  tag: string;
  subtitle: string;
  /** Empty while the log has not caught up with a live result. */
  when: string;
}

export function cardHeader(
  kind: LogKind,
  model: CardModel,
  entry: LogEntry | null,
  t: TFunction,
  lang: string,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): CardHeader {
  const tag = t(`log.kinds.${kind}`, { defaultValue: kind });
  let subtitle = "";
  switch (model.layout) {
    case "attack":
      subtitle = attackWeapon(model.roll, t, names);
      break;
    case "save":
      subtitle = t(`saves.categories.${model.save.category}`, {
        defaultValue: model.save.category,
      });
      break;
    case "cast":
      subtitle = spellName(model.cast.spellId, model.cast.spell);
      break;
    case "castLive":
      subtitle = spellName(model.result.spellId, model.result.name);
      break;
    case "exposure":
      subtitle = spellName(model.exposure.spellId, model.exposure.name);
      break;
    case "item":
      subtitle = names.item(model.item.itemId, model.item.name);
      break;
    case "initiative":
      subtitle = turnLabel(t, model.round);
      break;
    case "skill":
      subtitle = skillName(t, model.check.id);
      break;
    case "dice":
      subtitle = model.roll.formula;
      break;
    case "info":
      break;
  }
  const when = entry
    ? t(entry.round > 0 ? "fil.when" : "fil.whenOutOfCombat", {
        round: entry.round,
        time: formatTime(entry.createdAt, lang),
      })
    : "";
  return { tag, subtitle, when };
}

/** The value column of a journal row: the number that sums the entry up. */
export interface RowValue {
  text: string;
  tone: "damage" | "heal" | "pass" | "fail" | "accent" | "muted";
}

export function rowValue(e: LogEntry): RowValue | null {
  const hp = hpValue(e);
  if (hp) return { text: formatSigned(hp.amount), tone: hp.tone };
  const attack = attackPayload(e);
  if (attack) {
    if (!attackSettled(attack)) return { text: "?", tone: "muted" };
    return attack.hit
      ? { text: String(attack.totalDamage), tone: "accent" }
      : { text: "–", tone: "muted" };
  }
  const save = savePayload(e);
  if (save)
    return { text: String(save.d20Roll), tone: save.passed ? "pass" : "fail" };
  const item = itemPayload(e);
  if (item)
    return item.dice ? { text: String(item.total), tone: "accent" } : null;
  const initiative = initiativePayload(e);
  if (initiative) return { text: String(initiative.d10), tone: "accent" };
  const skill = skillCheckPayload(e);
  if (skill)
    return { text: String(skill.roll), tone: skill.success ? "pass" : "fail" };
  const dice = dicePayload(e);
  if (dice) return { text: String(dice.total), tone: "accent" };
  const exposure = exposurePayload(e);
  if (exposure) {
    if (!exposure.onSelf)
      return { text: String(exposure.damage.total), tone: "accent" };
    // On the Bard: the HP the server applied (the floor may cut it), as an HP change shows it.
    const amount = e.status === "voided" ? e.hpDelta : e.effectiveHpDelta;
    return { text: formatSigned(amount), tone: "damage" };
  }
  const cast = castPayload(e);
  if (cast?.damage) return { text: String(cast.damage.total), tone: "accent" };
  if (cast?.attack)
    return {
      text: String(cast.attack.d20),
      tone: cast.attack.hit ? "pass" : "fail",
    };
  return null;
}

/**
 * The header's last result (ADR 0022 step 2.2): the newest result entry (RESULT_KINDS) or free dice
 * roll still in force, so a roll made from any activity shows there. Undone entries give way to the
 * one before them.
 */
export function lastResultEntry(entries: readonly LogEntry[]): LogEntry | null {
  return (
    entries.find(
      (e) =>
        e.status !== "voided" &&
        (RESULT_KINDS.includes(e.kind) || e.kind === "dice"),
    ) ?? null
  );
}

/**
 * The chip of the header's last result: an attack's d20 (its wording already carries the verdict
 * and the damage), else the journal row's value.
 */
export function lastResultChip(e: LogEntry): RowValue | null {
  const attack = attackPayload(e);
  if (attack) return { text: String(attack.d20Roll), tone: "muted" };
  return rowValue(e);
}

/**
 * The faces of a single dice group ("6d6", "1d8+2" → 6, 8), to mark a die at its maximum or at 1;
 * null for anything else ("2d6+1d4").
 */
export function dieFaces(dice: string): number | null {
  const m = /^\s*\d*\s*d\s*(\d+)\s*([+-]\s*\d+\s*)?$/i.exec(dice);
  return m ? Number(m[1]) : null;
}

/** One die as a chip: its value and emphasis. */
export interface DieChipView {
  value: number;
  tone: "max" | "min" | "plain";
}

function chips(
  dice: readonly { sides: number; value: number }[],
): DieChipView[] {
  return dice.map((d) => ({ value: d.value, tone: dieTone(d.value, d.sides) }));
}

/**
 * The attack's damage dice as chips, from the roll as recorded (the payload or the roll endpoint);
 * null when it has none (a miss, or an attack logged before they were recorded): the card then
 * shows "dice -> total".
 */
export function attackDiceChips(
  r: Pick<RollResult, "damageRolls">,
): DieChipView[] | null {
  return r.damageRolls && r.damageRolls.length > 0
    ? chips(r.damageRolls)
    : null;
}

/** One damage instance of a cast: its dice as chips and its total; `parts` when the damage has parts. */
export interface CastInstanceView {
  dice: DieChipView[];
  total: number;
  parts?: CastPartView[];
  /** The round or instance of a damage made of dice steps; absent without steps. */
  step?: number;
}

/** One part of a damage instance: its dice as chips (none for a flat part), its total and what a save does to it. */
export interface CastPartView {
  dice: DieChipView[];
  total: number;
  save: string;
}

/**
 * The dice instances of a cast's damage, from its payload; null on a cast logged before they were
 * recorded (the card then shows the total only).
 */
export function castInstances(c: CastLogPayload): CastInstanceView[] | null {
  const rolls = c.damage?.rolls;
  if (!rolls || rolls.length === 0) return null;
  const saves = c.damage?.parts;
  return rolls.map((r) => {
    const view: CastInstanceView = { dice: chips(r.dice), total: r.total };
    if (r.step) view.step = r.step;
    if (r.parts && saves && r.parts.length === saves.length) {
      view.parts = r.parts.map((p, i) => ({
        dice: chips(p.dice),
        total: p.total,
        save: saves[i].save,
      }));
    }
    return view;
  });
}

/** A die's emphasis: its maximum in the accent, a 1 dimmed. */
export function dieTone(
  value: number,
  faces: number | null,
): "max" | "min" | "plain" {
  if (faces !== null && value === faces) return "max";
  if (value === 1) return "min";
  return "plain";
}
