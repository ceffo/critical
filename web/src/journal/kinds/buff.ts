// The buff and round entries (buffsvc logPayload, ADR 0004 and ADR 0005): buff_change, round_advance,
// round_reset, round_set and time_passed share one payload, worded by its action.
import type { TFunction } from "i18next";
import { Clock, Hourglass, Sparkle, type Icon } from "@phosphor-icons/react";
import { buffName, type SpellNamer } from "@/i18n/spellName";
import type { InventoryNamer } from "@/i18n/inventoryName";
import type { LocalizedText } from "@/types/localized";
import type { EndCause, EndEffectRoll } from "@/types/buffs";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { counters } from "../words";

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
  /** The buff's proficiency when a successful check put it on (ADR 0037 step 20). */
  proficiencyId?: string;
  /** Parallel to `expired`: each expired buff's proficiency, "" when none; absent when none is. */
  expiredProficiencyIds?: string[];
  note?: string;
  fromNote?: string;
  noteChanged?: boolean;
  /** dismiss: how the player said the row ended, when its spell has end effects (ADR 0004 step 21). */
  cause?: EndCause;
  /** The end effects the action applied, and current HP around them (ADR 0004 step 21). */
  endEffects?: EndEffectRoll[];
  hpBefore?: number;
  hpAfter?: number;
  /** dismiss: the psionic power the stopped row kept up (ADR 0043 step 16). */
  powerId?: string;
  /** dismiss: the mind a tangent or contact row held, when the player let go of it (ADR 0044 step 3). */
  target?: string;
  /**
   * next_round / pass_time: what each maintained power was charged (ADR 0043 steps 14 and 15). A
   * tangent or contact row's charge has no power (`powerId` ""), and names its `target` (ADR 0044).
   */
  maintenance?: { powerId: string; target?: string; cost: number }[];
  /** next_round / pass_time: the minds whose tangent or contact row the PSPs left could not pay. */
  unpaidTargets?: string[];
  /** next_round / pass_time: the powers the PSPs left could not pay; they ended. */
  maintenanceUnpaid?: string[];
  /** The powers above (and a dismissed one) by id, named in every language when logged. */
  powerNames?: Record<string, LocalizedText>;
}

/** A power named from its id when the entry has no name: "mind_thrust" is "Mind thrust". */
export function powerLabel(id: string): string {
  const words = id.replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** The dice of one end effect as rolled: "4", "4 + 2", "7 + 1" (a constant last). */
function rolledText(e: EndEffectRoll): string {
  const parts = e.rolls.map((r) => String(r.value));
  if (e.constant) parts.push(String(e.constant));
  return parts.join(" + ");
}

/**
 * What the end effects of the rows that ended did (ADR 0004 step 21): each effect's heal with its
 * dice, then the HP change. `named` puts the row's name before each effect (several rows may have
 * ended: Next Round, passing time, the notice); a dismiss names its one row already. Shared by the
 * journal and the notice after the action. "" when no effect applied.
 */
export function endEffectsText(
  t: TFunction,
  spellName: SpellNamer,
  effects: readonly EndEffectRoll[] | undefined,
  hpBefore: number | undefined,
  hpAfter: number | undefined,
  named: boolean,
): string {
  if (!effects || effects.length === 0) return "";
  // The cap at max HP gave nothing: the roll shows, not a gain that did not happen.
  const capped = hpBefore !== undefined && hpBefore === hpAfter;
  const key = capped
    ? named
      ? "log.buff.endCappedNamed"
      : "log.buff.endCapped"
    : named
      ? "log.buff.endHealNamed"
      : "log.buff.endHeal";
  const heals = effects
    .map((e) =>
      t(key, {
        buff: buffName(spellName, { name: e.buff, spellId: e.spellId }),
        total: e.total,
        dice: e.dice,
        rolls: rolledText(e),
      }),
    )
    .join("; ");
  return hpBefore !== undefined && hpAfter !== undefined
    ? t("log.buff.endHp", { heals, before: hpBefore, after: hpAfter })
    : heals;
}

function decodeBuff(e: LogEntry): BuffLogPayload | null {
  const p = e.payload as Partial<BuffLogPayload> | null;
  return p && typeof p.action === "string" ? (p as BuffLogPayload) : null;
}

function buffText(
  p: BuffLogPayload,
  t: TFunction,
  spellName: SpellNamer,
  e: LogEntry,
  names: InventoryNamer,
): string {
  // The name the entry recorded, in the player's language; the id prettified for an older entry.
  const power = (id: string, fallback = powerLabel(id)) =>
    names.text(p.powerNames?.[id], fallback);
  const buff = buffName(
    spellName,
    {
      name: p.buff ?? "",
      spellId: p.spellId,
      proficiencyId: p.proficiencyId,
    },
    t,
  );
  const expired = (p.expired ?? []).map((name, i) =>
    buffName(
      spellName,
      {
        name,
        spellId: p.expiredSpellIds?.[i],
        proficiencyId: p.expiredProficiencyIds?.[i],
      },
      t,
    ),
  );
  // ADR 0004 step 21: a dismiss names its one row; the rows that run out are named by effect.
  const endOfOne = endEffectsText(
    t,
    spellName,
    p.endEffects,
    p.hpBefore,
    p.hpAfter,
    false,
  );
  const endOfMany = endEffectsText(
    t,
    spellName,
    p.endEffects,
    p.hpBefore,
    p.hpAfter,
    true,
  );
  const withEnded = (text: string) =>
    endOfOne ? t("log.buff.withEnd", { text, end: endOfOne }) : text;
  // ADR 0043 steps 14 and 15: what the maintained powers cost, and the ones that could not be paid.
  const kept = (p.maintenance ?? [])
    .map((m) =>
      t("log.buff.maintenanceOne", {
        power: m.target && !m.powerId ? m.target : power(m.powerId),
        cost: m.cost,
      }),
    )
    .join(", ");
  const withExpired = (text: string) => {
    let out =
      expired.length > 0
        ? `${text} ${t("log.buff.expired", { names: expired.join(", ") })}`
        : text;
    if (kept) out = `${out} ${t("log.buff.maintenance", { powers: kept })}`;
    if (p.maintenanceUnpaid?.length) {
      out = `${out} ${t("log.buff.powersEnded", {
        powers: p.maintenanceUnpaid.map((id) => power(id)).join(", "),
      })}`;
    }
    if (p.unpaidTargets?.length) {
      out = `${out} ${t("log.buff.targetsEnded", {
        targets: p.unpaidTargets.join(", "),
      })}`;
    }
    return endOfMany
      ? t("log.buff.withEndAfter", { text: out, end: endOfMany })
      : out;
  };
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
      if (p.target && !p.powerId)
        return t("log.buff.letGo", { target: p.target });
      if (p.powerId)
        return t("log.buff.powerStopped", { power: power(p.powerId, buff) });
      return withEnded(
        p.cause === "voluntary"
          ? t("log.buff.dismissVoluntary", { buff })
          : p.cause === "dispelled"
            ? t("log.buff.dismissDispelled", { buff })
            : t("log.buff.dismiss", { buff }),
      );
    case "next_round":
      return withExpired(
        t("log.buff.nextRound", {
          from: p.fromRound ?? 0,
          to: p.toRound ?? 0,
        }),
      );
    case "pass_time":
      return withExpired(
        t("log.buff.passTime", {
          hours: p.hours ?? 0,
          rounds: t("buffs.rounds", { count: p.rounds ?? 0 }),
          from: p.fromRound ?? 0,
          to: p.toRound ?? 0,
        }),
      );
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

function buffKind(icon: Icon): KindView<BuffLogPayload> {
  return {
    decode: decodeBuff,
    icon,
    text: (p, w, e) => buffText(p, w.t, w.spellName, e, w.names),
  };
}

export const buffChangeKind = buffKind(Sparkle);
export const roundAdvanceKind = buffKind(Hourglass);
export const roundResetKind = buffKind(Hourglass);
export const roundSetKind = buffKind(Hourglass);
export const timePassedKind = buffKind(Clock);
