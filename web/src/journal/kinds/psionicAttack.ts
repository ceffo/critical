// An attack mode's use (psionicsvc, ADR 0044 step 7): its one or two checks, each against the
// opponent's defence. The words are built from the payload as logged; who won each contest is the
// server's.
import { Lightning } from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import type { LocalizedText } from "@/types/localized";
import { checkText } from "@/components/powers/defend";
import type { KindView, ShortVerdict } from "../kindView";

/** One check as logged (psionicsvc AttackCheck). */
export interface AttackCheckPayload {
  check: { target: number; d20: number; outcome: string; kept?: boolean };
  defence?: { roll: number; success: boolean };
  winner: "attacker" | "defender";
  effect?: boolean;
}

/** The attack entry as logged (psionicsvc AttackRecord). */
export interface PsionicAttackPayload {
  power: string;
  name?: LocalizedText;
  score: number;
  target: string;
  mind: "open" | "closed";
  defence?: string;
  defenceName?: LocalizedText;
  attackVersusDefence?: number;
  checks: AttackCheckPayload[];
  cost: number;
  spent: number;
  tangentsBefore: number;
  tangentsAfter: number;
  contact?: boolean;
  broken?: string;
  warnings?: string[];
}

function verdict(p: PsionicAttackPayload, t: TFunction): ShortVerdict {
  return p.checks.some((c) => c.winner === "attacker")
    ? { text: t("log.contest.attackerWon"), tone: "pass" }
    : { text: t("log.contest.defended"), tone: "fail" };
}

export const psionicAttackKind: KindView<PsionicAttackPayload> = {
  decode(e) {
    const p = e.payload as Partial<PsionicAttackPayload> | null;
    return p &&
      typeof p.power === "string" &&
      typeof p.target === "string" &&
      Array.isArray(p.checks)
      ? (p as PsionicAttackPayload)
      : null;
  },
  icon: Lightning,
  // "Psionic attack (Ego Whip on Orc): d20 5 vs 13 (Success): attacker won; … (tangents 1 → 2, PSPs 4)".
  text: (p, { t, names }) => {
    const results = p.checks
      .map((c) =>
        t("log.contest.checkWon", {
          check: checkText(c.check, false, t),
          won: t(
            c.winner !== "attacker"
              ? "log.contest.defended"
              : c.effect
                ? "log.contest.attackerWonEffect"
                : "log.contest.attackerWon",
          ),
        }),
      )
      .reduce(
        (a, b) => (a ? t("log.contest.andThen", { first: a, next: b }) : b),
        "",
      );
    const parts: string[] = [];
    // Contact ends the tangents' count; a closed mind otherwise shows how many it holds now.
    if (p.contact) parts.push(t("log.attackParts.contact"));
    else if (p.tangentsBefore !== p.tangentsAfter)
      parts.push(
        t("log.attackParts.tangents", {
          from: p.tangentsBefore,
          to: p.tangentsAfter,
        }),
      );
    if (p.broken) parts.push(t("log.attackParts.broken", { target: p.broken }));
    parts.push(
      p.spent < p.cost
        ? t("log.powerParts.halfCost", { spent: p.spent, cost: p.cost })
        : t("log.attackParts.cost", { spent: p.spent }),
    );
    for (const w of p.warnings ?? [])
      parts.push(t(`log.powerParts.warning.${w}`));
    return t("log.psionicAttackWith", {
      text: t("log.psionicAttack", {
        attack: names.text(p.name, p.power),
        target: p.target,
        results,
      }),
      parts: parts.join(", "),
    });
  },
  // The app rolls a d20 per check.
  rolled: () => true,
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.psionic_attack"),
    verdict: verdict(p, t),
  }),
};
