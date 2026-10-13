import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import type { RollResult } from "@/types/combat";
import { spellName } from "@/i18n/spellName";
import { formatSigned } from "@/lib/format";

/** Payload of cast entries (castsvc castRecord). */
export interface CastLogPayload {
  action: "cast" | "received";
  spellId: string;
  spell: string;
  casterLevel: number;
  buff?: { name: string; rounds?: number; charges?: number; strGain?: number; applied: boolean };
  damage?: { total: number; type: string };
}

function castPayload(e: LogEntry): CastLogPayload | null {
  if (e.kind !== "cast") return null;
  const p = e.payload as Partial<CastLogPayload> | null;
  return p && typeof p.action === "string" && typeof p.spellId === "string"
    ? (p as CastLogPayload)
    : null;
}

function castText(p: CastLogPayload, t: TFunction): string {
  const spell = spellName(t, p.spellId, p.spell);
  const head =
    p.action === "received"
      ? t("log.cast.received", { spell, level: p.casterLevel })
      : t("log.cast.cast", { spell });
  const details: string[] = [];
  if (p.buff?.applied) {
    details.push(counters(t, p.buff.rounds, p.buff.charges));
    if (p.buff.strGain) details.push(t("log.cast.strGain", { gain: p.buff.strGain }));
  }
  if (p.damage) details.push(t("log.cast.damage", { total: p.damage.total, type: p.damage.type }));
  return details.length > 0 ? `${head}: ${details.join(", ")}` : head;
}
import type { UseResult } from "@/types/items";

/** Payload of long_rest entries (restsvc restRecord). */
function restText(e: LogEntry, t: TFunction): string | null {
  if (e.kind !== "long_rest") return null;
  const p = e.payload as { slotsReset?: number; itemsRefilled?: string[] } | null;
  if (!p || typeof p.slotsReset !== "number") return null;
  return t("log.rest", { slots: p.slotsReset, items: p.itemsRefilled?.length ?? 0 });
}

/** Payload of buff_change / round_advance entries (buffsvc logPayload). */
export interface BuffLogPayload {
  action: "activate" | "edit" | "absorb" | "dismiss" | "next_round" | "end_combat" | "set_round";
  buff?: string;
  rounds?: number;
  charges?: number;
  fromRounds?: number;
  fromCharges?: number;
  ended?: boolean;
  fromRound?: number;
  toRound?: number;
  expired?: string[];
}

function buffPayload(e: LogEntry): BuffLogPayload | null {
  if (e.kind !== "buff_change" && e.kind !== "round_advance" && e.kind !== "round_reset" && e.kind !== "round_set") {
    return null;
  }
  const p = e.payload as Partial<BuffLogPayload> | null;
  return p && typeof p.action === "string" ? (p as BuffLogPayload) : null;
}

/** "5 rd, 3 ch", or "until dismissed" when the buff has neither counter. */
function counters(t: TFunction, rounds = 0, charges = 0): string {
  const parts = [
    rounds > 0 ? t("buffs.rounds", { count: rounds }) : null,
    charges > 0 ? t("buffs.charges", { count: charges }) : null,
  ].filter((p): p is string => p !== null);
  return parts.length > 0 ? parts.join(", ") : t("buffs.untilDismissed");
}

/** Payload of attack entries: the roll breakdown (combatsvc attackRecord, same shape as RollResult). */
function attackPayload(e: LogEntry): RollResult | null {
  if (e.kind !== "attack") return null;
  const p = e.payload as Partial<RollResult> | null;
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
}

function savePayload(e: LogEntry): SaveLogPayload | null {
  if (e.kind !== "save") return null;
  const p = (e.effectivePayload ?? e.payload) as Partial<SaveLogPayload> | null;
  return p && typeof p.passed === "boolean" && typeof p.d20Roll === "number" && typeof p.category === "string"
    ? (p as SaveLogPayload)
    : null;
}

function saveText(p: SaveLogPayload, t: TFunction): string {
  const verdict = (passed: boolean) => t(passed ? "result.passed" : "result.failed").toUpperCase();
  const args = {
    name: t(`saves.categories.${p.category}`, { defaultValue: p.category }),
    d20: p.d20Roll,
    target: p.target,
  };
  if (p.previousPassed === undefined) return t("log.saveThrow.rolled", { ...args, verdict: verdict(p.passed) });
  const change = { ...args, from: verdict(p.previousPassed), to: verdict(p.passed) };
  if (p.adjustment === 0) return t("log.saveThrow.removed", change);
  return t("log.saveThrow.adjusted", {
    ...change,
    adjustment: formatSigned(p.adjustment),
    reason: p.adjustmentReason ? ` ${p.adjustmentReason}` : "",
  });
}

/** Payload of item_use entries (itemsvc useRecord, same shape as UseResult). */
function itemPayload(e: LogEntry): UseResult | null {
  if (e.kind !== "item_use") return null;
  const p = e.payload as Partial<UseResult> | null;
  return p && typeof p.name === "string" && typeof p.remaining === "number" ? (p as UseResult) : null;
}

function itemText(p: UseResult, t: TFunction): string {
  const args = { name: p.name, dice: p.dice, total: p.total, left: p.remaining };
  return t(p.dice ? "log.item.rolled" : "log.item.used", args);
}

function attackText(p: RollResult, t: TFunction): string {
  const weapon = p.ammo
    ? t("log.attack.withAmmo", { weapon: p.weaponName, ammo: p.ammo.name })
    : (p.weaponName ?? "");
  const args = { weapon, ac: p.targetAc, damage: p.totalDamage, mult: p.critMultiplier };
  if (!p.hit) return t("log.attack.miss", args);
  return t(p.isCrit ? "log.attack.crit" : "log.attack.hit", args);
}

/**
 * The entry's text in the player's language: buff, round, attack, save and cast entries are rendered from
 * their structured payload; everything else (and unknown payloads) uses the server's summary.
 */
export function entryText(e: LogEntry, t: TFunction): string {
  const attack = attackPayload(e);
  if (attack) return attackText(attack, t);
  const save = savePayload(e);
  if (save) return saveText(save, t);
  const cast = castPayload(e);
  if (cast) return castText(cast, t);
  const rest = restText(e, t);
  if (rest) return rest;
  const item = itemPayload(e);
  if (item) return itemText(item, t);
  const p = buffPayload(e);
  if (!p) return e.effectiveSummary;
  const buff = p.buff ?? "";
  switch (p.action) {
    case "activate":
      return t("log.buff.activate", {
        buff,
        counters: counters(t, p.rounds, p.charges),
      });
    case "edit":
      return t(p.ended ? "log.buff.editEnded" : "log.buff.edit", {
        buff,
        from: counters(t, p.fromRounds, p.fromCharges),
        to: counters(t, p.rounds, p.charges),
      });
    case "absorb":
      return p.ended
        ? t("log.buff.absorbEnded", { buff })
        : t("log.buff.absorb", { buff, count: p.charges ?? 0 });
    case "dismiss":
      return t("log.buff.dismiss", { buff });
    case "next_round": {
      const text = t("log.buff.nextRound", {
        from: p.fromRound ?? 0,
        to: p.toRound ?? 0,
      });
      return p.expired && p.expired.length > 0
        ? `${text} ${t("log.buff.expired", { names: p.expired.join(", ") })}`
        : text;
    }
    case "end_combat":
      return t("log.buff.endCombat", { from: p.fromRound ?? 0, to: p.toRound ?? 0 });
    case "set_round":
      return t("log.buff.setRound", { from: p.fromRound ?? 0, to: p.toRound ?? 0 });
    default:
      return e.effectiveSummary;
  }
}
