import { useTranslation } from "react-i18next";
import { formula } from "@/lib/skillBudget";
import { cn } from "@/lib/utils";
import { formatAc, formatSigned } from "@/lib/format";
import type { RogueSkill, SkillBudget } from "@/types/skills";

/**
 * The terms a skill's percentage is made of, as text (ADR 0030 steps 17 and 18): base, points,
 * race, Dexterity and armour, their sum, and when the 95 cap or the 0 floor moved it, which one.
 * The figures are the server's; the only comparison here picks the word (cap or floor).
 */
export function SkillTerms({
  skill,
  cap = 0,
  className,
}: {
  skill: RogueSkill;
  /** The most points one skill holds (a thief's); 0 for no such limit. */
  cap?: number;
  className?: string;
}) {
  const { t } = useTranslation();
  const note =
    skill.rawPercent > skill.percent
      ? t("skills.capped", { max: skill.percent })
      : skill.rawPercent < skill.percent
        ? t("skills.floored", { min: skill.percent })
        : null;
  return (
    <span
      className={cn("text-label break-words text-muted-foreground", className)}
      data-testid={`skill-terms-${skill.id}`}
    >
      {t("skills.terms", {
        base: skill.base,
        points: skill.points,
        race: formatSigned(skill.race),
        dexterity: formatSigned(skill.dexterity),
        armor: formatSigned(skill.armor),
        raw: formatAc(skill.rawPercent),
      })}
      {note && <span data-testid={`skill-limit-${skill.id}`}>, {note}</span>}
      {cap > 0 && (
        <span data-testid={`skill-cap-${skill.id}`}>
          {" "}
          ({t("skills.pointsOf", { points: skill.points, cap })})
        </span>
      )}
    </span>
  );
}

/**
 * A thief's or a bard's skill point budget, always shown (ADR 0030 steps 17 and 18): the points
 * available with the terms of their formula, spent, left (in the warning colour when over), and a
 * thief's cap per skill with its formula. The figures are the server's.
 */
export function SkillBudgetSummary({
  budget,
  className,
}: {
  budget: SkillBudget;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <div
      className={cn("flex flex-col gap-0.5 px-2.5 text-body", className)}
      data-testid="skill-points-budget"
    >
      <p className="break-words" data-testid="skill-points-available">
        {t("skills.available", {
          level: budget.level,
          formula: formula(
            t,
            budget.first,
            budget.perLevel,
            budget.level,
            budget.available,
          ),
        })}
      </p>
      <p
        className={cn(
          "font-semibold tabular-nums",
          budget.left < 0 && "text-warning",
        )}
        data-over={budget.left < 0 ? "true" : undefined}
        data-testid="skill-points-left"
      >
        {t("skills.spentLeft", { spent: budget.spent, left: budget.left })}
      </p>
      {budget.perSkillCap > 0 && (
        <p
          className="break-words text-muted-foreground"
          data-testid="skill-points-cap"
        >
          {t("skills.perSkillCap", {
            cap: budget.perSkillCap,
            formula: formula(
              t,
              budget.perSkillFirst,
              budget.perSkillPerLevel,
              budget.level,
              budget.perSkillCap,
            ),
          })}
        </p>
      )}
    </div>
  );
}
