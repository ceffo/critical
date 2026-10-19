// The buff and round entries (buffsvc logPayload, ADR 0004 and ADR 0005): buff_change, round_advance,
// round_reset, round_set and time_passed share one payload, worded by its action.
import type { TFunction } from "i18next";
import { Clock, Hourglass, Sparkle, type Icon } from "@phosphor-icons/react";
import { buffName, type SpellNamer } from "@/i18n/spellName";
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
  note?: string;
  fromNote?: string;
  noteChanged?: boolean;
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
): string {
  const buff = buffName(spellName, { name: p.buff ?? "", spellId: p.spellId });
  const expired = (p.expired ?? []).map((name, i) =>
    buffName(spellName, { name, spellId: p.expiredSpellIds?.[i] }),
  );
  const withExpired = (text: string) =>
    expired.length > 0
      ? `${text} ${t("log.buff.expired", { names: expired.join(", ") })}`
      : text;
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
    text: (p, w, e) => buffText(p, w.t, w.spellName, e),
  };
}

export const buffChangeKind = buffKind(Sparkle);
export const roundAdvanceKind = buffKind(Hourglass);
export const roundResetKind = buffKind(Hourglass);
export const roundSetKind = buffKind(Hourglass);
export const timePassedKind = buffKind(Clock);
