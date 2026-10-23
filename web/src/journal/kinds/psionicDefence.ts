// A psychic contest against the character (psionicsvc, ADR 0044 steps 6 and 8): the DM's one or two
// rolls of an attack mode, each against the defence's check. The words are built from the payload as
// logged; who won each contest is the server's.
import { ShieldWarning } from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import type { LocalizedText } from "@/types/localized";
import { checkText } from "@/components/powers/defend";
import type { KindView, ShortVerdict } from "../kindView";

/** One contest as logged (psionicsvc ContestRecord). */
export interface ContestPayload {
  attack: { roll: number; success: boolean };
  check?: { target: number; d20: number; outcome: string; kept?: boolean };
  fromKept?: boolean;
  winner: "attacker" | "defender";
}

/** The contest entry as logged (psionicsvc DefenceRecord). */
export interface PsionicDefencePayload {
  attack: string;
  attackName?: LocalizedText;
  defence?: string;
  defenceName?: LocalizedText;
  defenceKind?: "raised" | "standing";
  ability?: string;
  abilityScore?: number;
  powerModifier?: number;
  relearns?: number;
  score?: number;
  kept?: boolean;
  /** Table 14's cell for the attack against the defence, for the DM's score. */
  modifier?: number;
  contests: ContestPayload[];
  keptCheck?: { target: number; d20: number; outcome: string; kept?: boolean };
}

function verdict(p: PsionicDefencePayload, t: TFunction): ShortVerdict {
  return p.contests.some((c) => c.winner === "attacker")
    ? { text: t("log.contest.attackerWon"), tone: "fail" }
    : { text: t("log.contest.defended"), tone: "pass" };
}

export const psionicDefenceKind: KindView<PsionicDefencePayload> = {
  decode(e) {
    const p = e.payload as Partial<PsionicDefencePayload> | null;
    return p && typeof p.attack === "string" && Array.isArray(p.contests)
      ? (p as PsionicDefencePayload)
      : null;
  },
  icon: ShieldWarning,
  // "Psychic contest (Psionic Blast against Mind Blank): defended, attacker won".
  text: (p, { t, names }) => {
    // Per contest: the check as decided (die, outcome, kept +1), then who won.
    const results = p.contests
      .map((c) => {
        const won = t(
          c.winner === "attacker"
            ? "log.contest.attackerWon"
            : "log.contest.defended",
        );
        return c.check
          ? t("log.contest.checkWon", {
              check: checkText(c.check, c.fromKept, t),
              won,
            })
          : won;
      })
      .reduce(
        (a, b) => (a ? t("log.contest.andThen", { first: a, next: b }) : b),
        "",
      );
    const kept =
      p.keptCheck && !p.contests.every((c) => c.fromKept)
        ? t("powers.defence.keptForRound", {
            d20: p.keptCheck.d20,
            target: p.keptCheck.target,
          })
        : "";
    return t(p.defence ? "log.psionicDefence" : "log.psionicDefenceNone", {
      attack: names.text(p.attackName, p.attack),
      defence: p.defence ? names.text(p.defenceName, p.defence) : "",
      results: kept
        ? t("log.contest.andThen", { first: results, next: kept })
        : results,
    });
  },
  // The app rolled a d20 per contest unless the check was kept or there was no defence.
  rolled: (p) => !!p?.contests.some((c) => c.check && !c.fromKept),
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.psionic_defence"),
    verdict: verdict(p, t),
  }),
};
