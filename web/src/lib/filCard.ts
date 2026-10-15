// The Fil column: which journal entry the detail card shows, what each entry kind renders from its
// payload, and the journal's filter. Display only: verdicts, totals and dice are the server's, read
// from the payloads as recorded (docs/all.md, "Core Architectural Rule").
import type { TFunction } from "i18next";
import type { HpBreakdown, LogEntry, LogKind } from "@/types/log";
import type { Modifier, RollResult } from "@/types/combat";
import type { UseResult } from "@/types/items";
import type { CastResult } from "@/types/spells";
import type { SkillCheck } from "@/types/skills";
import { spellName } from "@/i18n/spellName";
import { formatSigned, formatTime } from "./format";
import { attackSettled } from "./attackAdjust";
import {
  attackPayload,
  castPayload,
  entryText,
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
  | { kind: "skill"; result: SkillCheck };

/** The journal filter: everything, or rolls only. */
export type LogFilter = "all" | "rolls";

/**
 * The entries whose result is "the latest result", as the server's actionlog.RollKinds: the card
 * shows the newest of them by default, and only that one takes corrections (AC, save adjustment).
 */
export const RESULT_KINDS: readonly LogKind[] = ["attack", "save", "cast", "item_use", "skill_check"];

/** The newest entry of RESULT_KINDS, voided ones included (the server's "latest roll"). */
export function latestResult(entries: readonly LogEntry[]): LogEntry | null {
  return entries.find((e) => RESULT_KINDS.includes(e.kind)) ?? null;
}

/**
 * The "rolls" filter: entries that carry dice the server rolled (attacks, saves, item uses,
 * initiative, skill checks, and casts with a damage roll or a to-hit d20).
 */
export function isRollEntry(e: LogEntry): boolean {
  switch (e.kind) {
    case "attack":
    case "save":
    case "skill_check":
    case "item_use":
    case "initiative":
      return true;
    case "cast": {
      const p = castPayload(e);
      return p !== null && (p.damage !== undefined || p.attack !== undefined);
    }
    default:
      return false;
  }
}

export function filterEntries(entries: LogEntry[], filter: LogFilter): LogEntry[] {
  return filter === "all" ? entries : entries.filter(isRollEntry);
}

/** The log entry recording a live result. */
export function liveEntryId(last: LiveResult): number | undefined {
  return last.kind === "cast" ? last.result.entryId : last.result.logEntryId;
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
  if (selectedId !== null && selectedId !== latest?.id && selectedId !== liveId) {
    const selected = entries.find((e) => e.id === selectedId);
    if (selected) return { kind: "entry", entry: selected, latest: false };
  }
  if (last && (latest === null || liveId === undefined || liveId >= latest.id)) {
    return { kind: "live", last, entry: entries.find((e) => e.id === liveId) ?? null };
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
  | { layout: "item"; item: UseResult }
  | { layout: "initiative"; round: number; d10: number }
  | { layout: "skill"; check: SkillCheck }
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
  if (typeof p?.hpRequested !== "number" || typeof p.hpApplied !== "number") return null;
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
export function cardModel(e: LogEntry, t: TFunction): CardModel {
  const attack = attackPayload(e);
  if (attack) return { layout: "attack", roll: { ...attack, logEntryId: e.id } };
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
  const item = itemPayload(e);
  if (item) return { layout: "item", item: { ...item, logEntryId: e.id } };
  const initiative = initiativePayload(e);
  if (initiative) return { layout: "initiative", ...initiative };
  const skill = skillCheckPayload(e);
  if (skill) return { layout: "skill", check: { ...skill, logEntryId: e.id } };
  return {
    layout: "info",
    info: { text: entryText(e, t), value: hpValue(e), capped: cappedHeal(e) },
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
): CardHeader {
  const tag = t(`log.kinds.${kind}`, { defaultValue: kind });
  let subtitle = "";
  switch (model.layout) {
    case "attack":
      subtitle = model.roll.ammo
        ? t("log.attack.withAmmo", { weapon: model.roll.weaponName, ammo: model.roll.ammo.name })
        : (model.roll.weaponName ?? "");
      break;
    case "save":
      subtitle = t(`saves.categories.${model.save.category}`, {
        defaultValue: model.save.category,
      });
      break;
    case "cast":
      subtitle = spellName(t, model.cast.spellId, model.cast.spell);
      break;
    case "castLive":
      subtitle = spellName(t, model.result.spellId, model.result.name);
      break;
    case "item":
      subtitle = model.item.name;
      break;
    case "initiative":
      subtitle = turnLabel(t, model.round);
      break;
    case "skill":
      subtitle = skillName(t, model.check.id);
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
  if (save) return { text: String(save.d20Roll), tone: save.passed ? "pass" : "fail" };
  const item = itemPayload(e);
  if (item) return item.dice ? { text: String(item.total), tone: "accent" } : null;
  const initiative = initiativePayload(e);
  if (initiative) return { text: String(initiative.d10), tone: "accent" };
  const skill = skillCheckPayload(e);
  if (skill) return { text: String(skill.roll), tone: skill.success ? "pass" : "fail" };
  const cast = castPayload(e);
  if (cast?.damage) return { text: String(cast.damage.total), tone: "accent" };
  if (cast?.attack) return { text: String(cast.attack.d20), tone: cast.attack.hit ? "pass" : "fail" };
  return null;
}

/**
 * The faces of a single dice group ("6d6", "1d8+2" → 6, 8), to mark a die at its maximum or at 1;
 * null for anything else ("2d6+1d4").
 */
export function dieFaces(dice: string): number | null {
  const m = /^\s*\d*\s*d\s*(\d+)\s*([+-]\s*\d+\s*)?$/i.exec(dice);
  return m ? Number(m[1]) : null;
}

/** A die's emphasis: its maximum in the accent, a 1 dimmed. */
export function dieTone(value: number, faces: number | null): "max" | "min" | "plain" {
  if (faces !== null && value === faces) return "max";
  if (value === 1) return "min";
  return "plain";
}
