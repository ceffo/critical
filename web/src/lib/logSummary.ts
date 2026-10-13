import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import type { RollResult } from "@/types/combat";
import type { UseResult } from "@/types/items";

/** Payload of buff_change / round_advance entries (buffsvc logPayload). */
export interface BuffLogPayload {
  action: "activate" | "edit" | "absorb" | "dismiss" | "next_round";
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
  if (e.kind !== "buff_change" && e.kind !== "round_advance") return null;
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
 * The entry's text in the player's language: buff, round and attack entries are rendered from
 * their structured payload; everything else (and unknown payloads) uses the server's summary.
 */
export function entryText(e: LogEntry, t: TFunction): string {
  const attack = attackPayload(e);
  if (attack) return attackText(attack, t);
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
    default:
      return e.effectiveSummary;
  }
}
