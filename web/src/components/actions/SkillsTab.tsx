import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { Button } from "@/components/ui/button";
import { ThiefPointsEditor } from "./ThiefPointsEditor";
import { useCharacterStore } from "@/stores/character";
import { skillName } from "@/lib/logSummary";
import { formatSigned } from "@/lib/format";
import type { SkillKind } from "@/types/skills";

const HEADINGS: Record<SkillKind, string> = {
  proficiency: "skills.proficiencies",
  bardSkill: "skills.bardSkills",
  thiefSkill: "skills.thiefSkills",
};

/**
 * Skills tab of the Act panel (ADR 0014; layout ADR 0027 step 12): the non-weapon proficiencies
 * (d20 at or under the target) and the bard or thief skills (d100 at or under the percentage; a
 * thief's computed by the server, ADR 0030 step 17, with a budget warning), side by side
 * when the panel holds two 16rem columns, stacked otherwise. Each row is one button that rolls the
 * check on the server; a proficiency shows its ability with the modifier as a pill, a bard skill its
 * percentage. The target, the verdict and the DM's adjustment are on the result card, after the
 * roll (ADR 0027 step 8.3, ADR 0014 step 6).
 */
export function SkillsTab() {
  const { t } = useTranslation();
  const skills = useSkillsStore((s) => s.skills);
  const fetchSkills = useSkillsStore((s) => s.fetchSkills);
  const { rollSkill, rolling } = useRollStore();
  const logEntries = useLogStore((s) => s.entries);
  const budget = useCharacterStore((s) => s.character?.derived.skillBudget);
  const [editingPoints, setEditingPoints] = useState(false);
  // A check, an undo or an effect (any new log state) can change the abilities in effect.
  useEffect(() => {
    void fetchSkills();
  }, [fetchSkills, logEntries]);

  if (!skills) return null;
  const rows = (
    kind: SkillKind,
    list: { id: string; value: ReactNode; unusable?: boolean }[],
  ) => {
    const label = t(HEADINGS[kind]);
    const id = `skills-${kind}`;
    return (
      <section aria-labelledby={id} className="flex min-w-0 flex-col gap-2">
        <h3 id={id} className="group-heading section-label px-2.5">
          {label}
        </h3>
        <ul aria-label={label} className="flex flex-col gap-1">
          {list.map(({ id: skill, value, unusable }) => {
            const name = skillName(t, skill);
            // ADR 0030 step 17 (PHB p.39): a thieving skill under 1% waits for points.
            const label = unusable
              ? t("skills.unusable", { name })
              : t("skills.rollLabel", { name });
            return (
              <li key={skill}>
                {/* The shared clickable row (the saves' rows): a raised stripe, hover and focus. */}
                <button
                  type="button"
                  className="grid min-h-7 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                  disabled={rolling || unusable}
                  aria-label={label}
                  title={label}
                  data-testid={`skill-${skill}`}
                  onClick={() => void rollSkill(skill)}
                >
                  <span className="break-words">{name}</span>
                  {value}
                </button>
              </li>
            );
          })}
        </ul>
      </section>
    );
  };

  return (
    <div
      className="grid grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] items-start gap-x-6 gap-y-4"
      data-testid="skills-blocks"
    >
      {rows(
        "proficiency",
        skills.proficiencies.map((p) => ({
          id: p.id,
          value: (
            <Ability
              ability={p.ability}
              score={p.abilityScore}
              modifier={p.modifier}
            />
          ),
        })),
      )}
      {(skills.bardSkills.length > 0 || skills.thiefSkills.length === 0) &&
        rows(
          "bardSkill",
          skills.bardSkills.map((b) => ({
            id: b.id,
            value: (
              <span className="text-right font-semibold whitespace-nowrap tabular-nums">
                {t("skills.percent", { value: b.percent })}
              </span>
            ),
          })),
        )}
      {skills.thiefSkills.length > 0 && (
        <div className="flex min-w-0 flex-col gap-2">
          {rows(
            "thiefSkill",
            // ADR 0030 step 17: the server's percentage; its terms in the tooltip.
            skills.thiefSkills.map((s) => ({
              id: s.id,
              unusable: s.percent < 1,
              value: (
                <span
                  className="text-right font-semibold whitespace-nowrap tabular-nums"
                  title={t("skills.thiefTerms", {
                    base: s.base,
                    points: s.points,
                    race: formatSigned(s.race),
                    dexterity: formatSigned(s.dexterity),
                    armor: formatSigned(s.armor),
                    max: s.maxPercent,
                  })}
                  data-testid={`thief-percent-${s.id}`}
                >
                  {t("skills.percent", { value: s.percent })}
                </span>
              ),
            })),
          )}
          {budget && budget.length > 0 && (
            <ul
              className="flex flex-col gap-1 px-2.5 text-body text-warning"
              data-testid="skill-budget"
            >
              {budget.map((w) => (
                <li key={w.code + (w.skill ?? "")} className="break-words">
                  {t(`skills.budget.${w.code}`, {
                    name: w.skill ? skillName(t, w.skill) : "",
                    points: w.points,
                    limit: w.limit,
                  })}
                </li>
              ))}
            </ul>
          )}
          {editingPoints ? (
            <ThiefPointsEditor
              skills={skills.thiefSkills}
              onDone={() => setEditingPoints(false)}
            />
          ) : (
            <Button
              type="button"
              size="sm"
              className="mx-2.5 w-fit"
              onClick={() => setEditingPoints(true)}
              data-testid="thief-points-edit"
            >
              {t("skills.spendPoints")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

/** "DEX 17" and, when the proficiency has one, its modifier as a pill ("DEX 17 [−1]"). */
function Ability({
  ability,
  score,
  modifier,
}: {
  ability: string;
  score: number;
  modifier: number;
}) {
  const { t } = useTranslation();
  return (
    <span className="flex items-center justify-end gap-1.5 whitespace-nowrap tabular-nums">
      <span className="text-muted-foreground">
        {t(`attr.${ability}`, { defaultValue: ability })} {score}
      </span>{" "}
      {modifier !== 0 && (
        <span
          className="chip min-w-7 px-1.5 text-center font-semibold"
          data-testid="skill-modifier"
        >
          {formatSigned(modifier)}
        </span>
      )}
    </span>
  );
}
