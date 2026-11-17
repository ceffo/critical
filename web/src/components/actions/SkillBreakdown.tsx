import { useTranslation } from "react-i18next";
import { LimitNote, type TermsRow } from "@/components/skills/SkillTermsTable";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";

const TERMS = ["base", "points", "race", "dexterity", "armor"] as const;

/**
 * The terms a skill's percentage is made of, a row of label-over-figure pairs that wraps on a narrow card (ADR 0030 steps 17 and 18): base,
 * points, race, Dexterity and armour, one column each, and when the 95 cap or the 0 floor moved the
 * percentage, which one, and a thief's points against the cap per skill ("20 of 30"). The figures are
 * the server's.
 */
export function SkillTerms({
  skill,
  cap = 0,
  className,
}: {
  skill: TermsRow;
  /** The most points one skill holds (a thief's); 0 for no such limit. */
  cap?: number;
  className?: string;
}) {
  const { t } = useTranslation();
  const value = {
    base: String(skill.base),
    points: String(skill.points),
    race: formatSigned(skill.race),
    dexterity: formatSigned(skill.dexterity),
    armor: formatSigned(skill.armor),
  };
  return (
    <span
      className={cn("flex min-w-0 flex-col", className)}
      data-testid={`skill-terms-${skill.id}`}
    >
      <dl className="flex flex-wrap gap-x-3 text-label text-muted-foreground">
        {TERMS.map((k) => (
          <div key={k} className="flex flex-col">
            <dt className="font-medium">{t(`skillsActivity.terms.${k}`)}</dt>
            <dd className="tabular-nums">{value[k]}</dd>
          </div>
        ))}
      </dl>
      <LimitNote row={skill} />
      {cap > 0 && (
        <span
          className="text-label text-muted-foreground"
          data-testid={`skill-cap-${skill.id}`}
        >
          {t("skills.pointsOf", { points: skill.points, cap })}
        </span>
      )}
    </span>
  );
}
