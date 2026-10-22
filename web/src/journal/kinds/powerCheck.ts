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
  // "Power check (Mind Thrust): d20 9 vs 13, success, PSPs -7".
  text: (p, { t, names }, e) =>
    t("log.powerCheck", {
      power: names.text(p.name, p.power),
      d20: p.d20,
      target: p.target,
      verdict: verdict(p, t).text,
      spent: formatSigned(spent(e)),
    }),
  value: (p) => ({ text: String(p.d20), tone: p.success ? "pass" : "fail" }),
  rolled: () => true,
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.power_check"),
    verdict: verdict(p, t),
  }),
};
