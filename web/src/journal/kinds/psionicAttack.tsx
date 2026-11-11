// An attack mode's use (psionicsvc, ADR 0044 step 7): its one or two checks, each against the
// opponent's defence. The words are built from the payload as logged; who won each contest is the
// server's.
import { Lightning } from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useInventoryName } from "@/i18n/inventoryName";
import type { LocalizedText } from "@/types/localized";
import { checkText } from "@/components/powers/defend";
import type { KindView, ShortVerdict } from "../kindView";
import { Fact, HexDie, ResultLine, Verdict } from "../parts";

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

/** Each check: the die against its target, then who won. */
function checkLines(p: PsionicAttackPayload, t: TFunction): string[] {
  return p.checks.map((c) =>
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
  );
}

/** What the attack did to the tangents, the cost paid and the warnings. */
function partLines(p: PsionicAttackPayload, t: TFunction): string[] {
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
  return parts;
}

/** The outcome the summary leads with: what the attack won, or that it was defended. */
function outcome(p: PsionicAttackPayload, t: TFunction): string {
  if (!p.checks.some((c) => c.winner === "attacker"))
    return t("log.contest.defended");
  if (p.contact) return t("log.attackParts.contact");
  if (p.tangentsAfter > p.tangentsBefore)
    return t("log.summary.tangents", { count: p.tangentsAfter });
  return t(
    p.checks.some((c) => c.winner === "attacker" && c.effect)
      ? "log.summary.effect"
      : "log.contest.attackerWon",
  );
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
  text: (p, { t, names }) =>
    t("log.psionicAttackWith", {
      text: t("log.psionicAttack", {
        attack: names.text(p.name, p.power),
        target: p.target,
        results: checkLines(p, t).reduce(
          (a, b) => (a ? t("log.contest.andThen", { first: a, next: b }) : b),
          "",
        ),
      }),
      parts: partLines(p, t).join(", "),
    }),
  // "Ego Whip on Orc: 2 tangents".
  summary: (p, { t, names }) =>
    t("log.summary.attack", {
      attack: names.text(p.name, p.power),
      target: p.target,
      outcome: outcome(p, t),
    }),
  detail: (p, { t }) => [...checkLines(p, t), ...partLines(p, t)],
  // The app rolls a d20 per check.
  rolled: () => true,
  subtitle: (p, { names }) => names.text(p.name, p.power),
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.psionic_attack"),
    verdict: verdict(p, t),
  }),
  card: ({ payload }) => <PsionicAttackCard p={payload} />,
};

/**
 * An attack mode's use as a card (ADR 0044 step 7): the verdict, then each check as a row of tiles
 * (the attacker's d20 against its target and outcome, the opponent's defence roll when it had one,
 * who won), the tangents' change and the PSPs spent. Every figure and winner is the record's.
 */
function PsionicAttackCard({ p }: { p: PsionicAttackPayload }) {
  const { t } = useTranslation();
  const names = useInventoryName();
  const won = p.checks.some((c) => c.winner === "attacker");
  const word = won ? t("result.psi.attackerWon") : t("result.psi.defended");
  const lead = p.checks[0];
  const notes = [
    ...(p.broken ? [t("log.attackParts.broken", { target: p.broken })] : []),
    ...(p.warnings ?? []).map((w) => t(`log.powerParts.warning.${w}`)),
  ];
  return (
    <div className="flex flex-col gap-3" data-testid="psionic-attack-result">
      <div className="flex items-center gap-3">
        {lead && (
          <HexDie
            value={lead.check.d20}
            label={t("journal.d20", { value: lead.check.d20 })}
          />
        )}
        <div className="min-w-0 flex-1">
          <ResultLine verdict={<Verdict ok={won} yes={word} no={word} />}>
            <span
              className="text-key tabular-nums"
              data-testid="psionic-attack-score"
            >
              {t("result.psi.score", { score: p.score })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground">
            {t("result.psi.heading", {
              power: names.text(p.name, p.power),
              target: p.target,
            })}
            {p.defence && (
              <>
                {" · "}
                {t("result.psi.against", {
                  defence: names.text(p.defenceName, p.defence),
                })}
              </>
            )}
          </div>
        </div>
      </div>
      {p.checks.map((c, i) => (
        <dl
          key={i}
          className="grid grid-cols-[repeat(auto-fit,minmax(6rem,1fr))] gap-2"
          data-testid="psionic-attack-check"
        >
          <Fact label={t("result.psi.attackRoll", { n: i + 1 })}>
            {t("result.psi.rolledVs", {
              d20: c.check.d20,
              target: c.check.target,
            })}
            <div className="text-label font-normal text-muted-foreground">
              {t(`powers.use.outcome.${c.check.outcome}`)}
              {c.check.kept && <> · {t("powers.defence.keptPlusOne")}</>}
            </div>
          </Fact>
          {c.defence && (
            <Fact label={t("result.psi.defenceRoll")}>
              {c.defence.roll}
              <div className="text-label font-normal text-muted-foreground">
                {t(c.defence.success ? "result.passed" : "result.failed")}
              </div>
            </Fact>
          )}
          <Fact label={t("result.psi.winner")}>
            <span data-testid="psionic-attack-winner" data-winner={c.winner}>
              {t(
                c.winner !== "attacker"
                  ? "result.psi.defended"
                  : c.effect
                    ? "result.psi.attackerWonEffect"
                    : "result.psi.attackerWon",
              )}
            </span>
          </Fact>
        </dl>
      ))}
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(6rem,1fr))] gap-2">
        <Fact label={t("result.psi.tangents")}>
          <span data-testid="psionic-attack-tangents">
            {p.contact
              ? t("log.attackParts.contact")
              : p.tangentsBefore === p.tangentsAfter
                ? p.tangentsAfter
                : t("result.psi.tangentsChange", {
                    from: p.tangentsBefore,
                    to: p.tangentsAfter,
                  })}
          </span>
        </Fact>
        <Fact label={t("result.psi.cost")}>
          {p.spent < p.cost
            ? t("result.psi.paidOf", { spent: p.spent, cost: p.cost })
            : p.spent}
        </Fact>
      </dl>
      {notes.length > 0 && (
        <ul className="flex flex-col gap-0.5 text-label text-muted-foreground">
          {notes.map((n) => (
            <li key={n} className="break-words">
              {n}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
