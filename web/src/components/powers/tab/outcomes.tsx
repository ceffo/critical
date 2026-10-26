import { useTranslation } from "react-i18next";
import type {
  AttackResult,
  DefendResult,
  PowerCheckResult,
} from "@/api/psionics";
import { formatSigned } from "@/lib/format";
import { checkText } from "../defend";

const box = "flex flex-col gap-0.5 rounded-md bg-raised px-3 py-2 text-body";

/** The check just made by a use: d20 against the target, the verdict, the PSPs spent, the upkeep set up, the warnings. */
export function UseOutcome({ result: r }: { result: PowerCheckResult }) {
  const { t } = useTranslation();
  return (
    <div className={box} role="status" data-testid="use-result">
      <span>
        {t("powers.use.rolled", { d20: r.d20, target: r.target })} ·{" "}
        <strong data-outcome={r.outcome}>
          {t(`powers.use.outcome.${r.outcome}`)}
        </strong>
      </span>
      <span className="text-label text-muted-foreground">
        {t("powers.use.spent", { amount: formatSigned(-r.spent) })}
        {r.success && r.maintenanceCost
          ? ` · ${t("powers.use.keptUp", {
              cost: r.maintenanceCost,
              count: r.maintenancePeriod ?? 1,
            })}`
          : ""}
      </span>
      <Warnings codes={r.warnings} testId="use-warning" />
    </div>
  );
}

function Warnings({ codes, testId }: { codes?: string[]; testId: string }) {
  const { t } = useTranslation();
  return (codes ?? []).map((w) => (
    <span key={w} className="text-label text-danger" data-testid={testId}>
      {t(`powers.use.warning.${w}`, { defaultValue: w })}
    </span>
  ));
}

/** The attack just made: each check and who won it, the effect, the tangents (or contact), the cost. */
export function AttackOutcome({ result: r }: { result: AttackResult }) {
  const { t } = useTranslation();
  return (
    <div className={box} role="status" data-testid="attack-result">
      {r.attackVersusDefence !== undefined && (
        <span className="text-label text-muted-foreground">
          {t("powers.defence.tableModifier", {
            modifier: formatSigned(r.attackVersusDefence),
          })}
        </span>
      )}
      {r.checks.map((c, i) => (
        <span key={i} data-winner={c.winner} data-effect={c.effect ?? false}>
          {t("powers.attack.check", { n: i + 1 })} ·{" "}
          {checkText(c.check, false, t)} ·{" "}
          <strong>
            {t(
              c.winner !== "attacker"
                ? "log.contest.defended"
                : c.effect
                  ? "log.contest.attackerWonEffect"
                  : "log.contest.attackerWon",
            )}
          </strong>
        </span>
      ))}
      <span className="text-label" data-testid="attack-tangents">
        {r.contact
          ? t("log.attackParts.contact")
          : t("powers.attack.tangents", {
              target: r.target,
              from: r.tangentsBefore,
              to: r.tangentsAfter,
            })}
        {r.broken && ` · ${t("log.attackParts.broken", { target: r.broken })}`}
      </span>
      <span className="text-label text-muted-foreground">
        {r.spent < r.cost
          ? t("log.powerParts.halfCost", { spent: r.spent, cost: r.cost })
          : t("powers.use.spent", { amount: formatSigned(-r.spent) })}
      </span>
      <Warnings codes={r.warnings} testId="attack-warning" />
    </div>
  );
}

/** The contests just decided: who won each, the defence's check, the kept check, Table 14's cell. */
export function DefendOutcome({
  result: r,
  powerName,
}: {
  result: DefendResult;
  powerName: (id: string) => string;
}) {
  const { t } = useTranslation();
  return (
    <div className={box} role="status" data-testid="defend-result">
      <span className="text-label text-muted-foreground">
        {r.defence
          ? t("powers.defence.against", { name: powerName(r.defence) })
          : t("powers.defence.noDefence")}
        {r.modifier !== undefined &&
          ` · ${t("powers.defence.tableModifier", { modifier: formatSigned(r.modifier) })}`}
      </span>
      {r.contests.map((c, i) => (
        <span key={i} data-winner={c.winner}>
          {t("powers.defence.contest", { n: i + 1, roll: c.attack.roll })}
          {c.check && ` · ${checkText(c.check, c.fromKept, t)}`} ·{" "}
          <strong>
            {t(
              c.winner === "attacker"
                ? "log.contest.attackerWon"
                : "log.contest.defended",
            )}
          </strong>
        </span>
      ))}
      {r.keptCheck && (
        <span
          className="text-label text-muted-foreground first-letter:uppercase"
          data-testid="defend-kept"
        >
          {t("powers.defence.keptForRound", {
            d20: r.keptCheck.d20,
            target: r.keptCheck.target,
          })}
        </span>
      )}
    </div>
  );
}
