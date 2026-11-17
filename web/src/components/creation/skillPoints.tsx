import { useTranslation } from "react-i18next";
import { SkillPointsBoard } from "@/components/skills/SkillPointsBoard";
import { localized } from "@/i18n/localized";
import { skillName } from "@/journal/words";
import { skillSetsOf, type StepProps } from "./flow";

/**
 * The creation's skill points step (ADR 0030 steps 17 and 18, ADR 0034, ADR 0040 step 3.5): one
 * board per skill set the draft's classes grant, the editor's own (the points left in a bar that
 * sticks to the top, the breakdown table with a slider and a typed box per skill). The rows and the
 * budget are the draft's answer (`POST /api/characters/draft`), asked again as the points change;
 * the web computes none. A refusal shows beside its skill, or under the board for the set's own.
 */
export function SkillPointsStep({
  options,
  draft,
  update,
  errorFor,
  answer,
}: StepProps) {
  const { t, i18n } = useTranslation();
  const sets = skillSetsOf(answer.latest);
  const read = answer.latest?.derived?.skillSets ?? [];
  return (
    <div className="flex min-w-0 flex-col gap-6">
      {sets.map(({ set, name, skills, budget }) => {
        const rows = (read.find((r) => r.set === set)?.skills ?? []).map(
          ({ skill, ...terms }) => ({ id: skill, ...terms }),
        );
        const setError = errorFor(`skillPoints.${set}`);
        return (
          <section
            key={set}
            className="flex min-w-0 flex-col gap-2"
            aria-label={t("creation.skillPoints.title", {
              className: localized(i18n.language, name) || set,
            })}
            data-testid={`${set}-skills-entry`}
          >
            <SkillPointsBoard
              setId={set}
              skillIds={skills}
              rows={rows}
              budget={budget}
              maxPercent={options.skillPoints.max}
              values={draft.skillPoints[set] ?? {}}
              onChange={(id, value) =>
                update({
                  skillPoints: {
                    ...draft.skillPoints,
                    [set]: { ...draft.skillPoints[set], [id]: value },
                  },
                })
              }
              errorFor={(id) => errorFor(`skillPoints.${set}.${id}`)}
              name={(id) => {
                const text = errorFor(`skillPoints.${set}.${id}`);
                return (
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="break-words">{skillName(t, id)}</span>
                    {text && (
                      <span
                        id={`skill-error-${set}-${id}`}
                        role="alert"
                        className="text-label break-words text-danger"
                      >
                        {text}
                      </span>
                    )}
                  </span>
                );
              }}
              header={
                sets.length > 1 ? (
                  <h3 className="px-2.5 text-body font-semibold">
                    {t("creation.skillPoints.name", {
                      className: localized(i18n.language, name) || set,
                    })}
                  </h3>
                ) : undefined
              }
            />
            {setError && (
              <p role="alert" className="text-label break-words text-danger">
                {setError}
              </p>
            )}
          </section>
        );
      })}
    </div>
  );
}
