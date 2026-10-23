// A psionicist's power check (psionicsvc, ADR 0043 steps 10 and 11): the d20 against the power's
// target and the PSPs it spent, as the entry's PSP delta. The power is named by the name the entry
// recorded, in the player's language (its id for an entry with none).
import { Brain } from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import { formatSigned } from "@/lib/format";
import type { LocalizedText } from "@/types/localized";
import type { LogEntry } from "@/types/log";
import type { KindView, ShortVerdict } from "../kindView";

/** The check's breakdown and outcome, as logged (psionicsvc CheckRecord). */
export interface PowerCheckPayload {
  power: string;
  /** The power's catalogue name when logged, in every language. */
  name?: LocalizedText;
  ability?: string;
  abilityScore?: number;
  powerModifier?: number;
  enteredScore?: number;
  relearns?: number;
  modifiers?: { value: number; reason?: string }[];
  target: number;
  d20: number;
  outcome: "success" | "failure" | "power_score" | "natural_20";
  success: boolean;
  cost: number;
  spent: number;
  /** A power kept up: what each charge costs and how many rounds apart (logged on a success only). */
  maintenanceCost?: number;
  maintenancePeriod?: number;
  /** Warnings the check raised, by code (second_power_this_round). */
  warnings?: string[];
  /** The mind the check was on: the contact power's target, or the contacted mind of a power paid on contact. */
  opponent?: string;
  /** The contact power's target's level (Table 29's row). */
  level?: number;
  /** The contact power's score lines (Tables 30 and 31, resisting), by code. */
  contactModifiers?: { kind: string; code: string; modifier: number }[];
  /** The contact power's success made (or renewed) the opponent's contact row. */
  contact?: boolean;
}

function verdict(p: PowerCheckPayload, t: TFunction): ShortVerdict {
  switch (p.outcome) {
    case "power_score":
      return { text: t("lastShort.verdict.powerScore"), tone: "crit" };
    case "natural_20":
      return { text: t("lastShort.verdict.natural20"), tone: "fail" };
    case "success":
      return { text: t("lastShort.verdict.success"), tone: "pass" };
    default:
      return { text: t("lastShort.verdict.failure"), tone: "fail" };
  }
}

const spent = (e: LogEntry): number => e.pspDelta ?? 0;

/**
 * How the target came about, as recorded: the ability and the power's modifier (or the score the
 * player entered), the relearns, each labelled modifier, then the cost paid when it is not the
 * full one, the maintenance a success set up and the warnings.
 */
export function breakdown(p: PowerCheckPayload, t: TFunction): string[] {
  const parts: string[] = [];
  if (p.enteredScore !== undefined) {
    parts.push(t("log.powerParts.entered", { score: p.enteredScore }));
  } else if (p.ability) {
    parts.push(
      t("log.powerParts.ability", {
        ability: t(`attr.${p.ability}`, { defaultValue: p.ability }),
        score: p.abilityScore ?? 0,
        modifier: formatSigned(p.powerModifier ?? 0),
      }),
    );
  }
  if (p.relearns)
    parts.push(t("log.powerParts.relearns", { count: p.relearns }));
  if (p.opponent) {
    parts.push(
      p.level
        ? `${t("log.powerParts.opponent", { target: p.opponent })}, ${t("log.powerParts.targetLevel", { level: p.level })}`
        : t("log.powerParts.opponent", { target: p.opponent }),
    );
  }
  for (const c of p.contactModifiers ?? []) {
    parts.push(
      t("log.powerParts.contactLine", {
        what: t(`log.contactWords.${c.kind}.${c.code}`, {
          defaultValue: c.code,
        }),
        modifier: formatSigned(c.modifier),
      }),
    );
  }
  for (const m of p.modifiers ?? []) {
    parts.push(
      m.reason
        ? t("log.powerParts.modifierWhy", {
            value: formatSigned(m.value),
            reason: m.reason,
          })
        : formatSigned(m.value),
    );
  }
  if (p.spent !== p.cost) {
    parts.push(t("log.powerParts.halfCost", { spent: p.spent, cost: p.cost }));
  }
  if (p.success && p.maintenanceCost) {
    parts.push(
      t("log.powerParts.maintained", {
        cost: p.maintenanceCost,
        count: p.maintenancePeriod ?? 1,
      }),
    );
  }
  if (p.contact) parts.push(t("log.powerParts.contactMade"));
  for (const w of p.warnings ?? []) {
    parts.push(t(`log.powerParts.warning.${w}`, { defaultValue: w }));
  }
  return parts;
}

export const powerCheckKind: KindView<PowerCheckPayload> = {
  decode(e) {
    const p = e.payload as Partial<PowerCheckPayload> | null;
    return p &&
      typeof p.power === "string" &&
      typeof p.d20 === "number" &&
      typeof p.outcome === "string"
      ? (p as PowerCheckPayload)
      : null;
  },
  icon: Brain,
  // "Power check (Mind Thrust): d20 9 vs 13, success, PSPs −7 (INT 18 −4, +1 relearned, −2 the DM)".
  text: (p, { t, names }, e) => {
    const line = t("log.powerCheck", {
      power: names.text(p.name, p.power),
      d20: p.d20,
      target: p.target,
      verdict: verdict(p, t).text,
      spent: formatSigned(spent(e)),
    });
    const parts = breakdown(p, t);
    return parts.length > 0
      ? t("log.powerCheckWith", { text: line, parts: parts.join(", ") })
      : line;
  },
  value: (p) => ({ text: String(p.d20), tone: p.success ? "pass" : "fail" }),
  rolled: () => true,
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.power_check"),
    verdict: verdict(p, t),
  }),
};
