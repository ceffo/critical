import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { skillName } from "@/lib/logSummary";
import { formatSigned } from "@/lib/format";
import type { SkillKind } from "@/types/skills";

/**
 * Skills tab of the Act panel (ADR 0014; layout ADR 0027 step 12): the non-weapon proficiencies
 * (d20 at or under the target) and the bard skills (d100 at or under the percentage), side by side
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
  // A check, an undo or an effect (any new log state) can change the abilities in effect.
  useEffect(() => {
    void fetchSkills();
  }, [fetchSkills, logEntries]);

  if (!skills) return null;
  const rows = (kind: SkillKind, list: { id: string; value: ReactNode }[]) => {
    const label = t(
      kind === "proficiency" ? "skills.proficiencies" : "skills.bardSkills",
    );
    const id = `skills-${kind}`;
    return (
      <section aria-labelledby={id} className="flex min-w-0 flex-col gap-2">
        <h3 id={id} className="section-label">
          {label}
        </h3>
        <ul aria-label={label} className="flex flex-col gap-1">
          {list.map(({ id: skill, value }) => {
            const name = skillName(t, skill);
            return (
              <li key={skill}>
                {/* The shared clickable row (the saves' rows): a raised stripe, hover and focus. */}
                <button
                  type="button"
                  className="grid min-h-7 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                  disabled={rolling}
                  aria-label={t("skills.rollLabel", { name })}
                  title={t("skills.rollLabel", { name })}
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
      {rows(
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
