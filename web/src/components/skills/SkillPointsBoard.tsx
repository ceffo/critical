import { useTranslation } from "react-i18next";
import { PoolBar } from "@/components/proficiencies/pickParts";
import { PointsSlider } from "@/components/skills/PointsSlider";
import {
  SkillTermsTable,
  type TermsRow,
} from "@/components/skills/SkillTermsTable";
import { skillName } from "@/journal/words";
import { formula } from "@/lib/skillBudget";
import { pointsLimit } from "@/lib/skillLimit";
import { cn } from "@/lib/utils";
import { useAbilityFocus } from "@/stores/abilityFocus";
import type { SkillBudget } from "@/types/skills";
import type { ReactNode } from "react";

/** The whole points typed: what a field that is not a whole number holds counts for nothing. */
function typedTotal(values: Record<string, string>, ids: string[]): number {
  let n = 0;
  for (const id of ids) {
    const v = Number(values[id]?.trim() ? values[id] : 0);
    if (Number.isInteger(v) && v > 0) n += v;
  }
  return n;
}

/**
 * A skill set's points, balanced (ADR 0030 steps 17 and 18, ADR 0027 step 1): the points left in a
 * bar that sticks to the top of the scroller, and the breakdown table with a slider and a typed box
 * in each skill's Points column. The editor and the creation step both draw it. Every figure
 * (the budget, the terms, the percentages) is the server's; the sliders stop at the skill's limit and
 * at what the points typed leave, and the server refuses anything else, worded beside the skill.
 */
export function SkillPointsBoard({
  setId,
  skillIds,
  rows,
  budget,
  maxPercent,
  values,
  onChange,
  errorFor,
  name,
  header,
  stale = false,
  testId = "skill-points-preview",
}: {
  setId: string;
  /** The set's skills in their order; a row is read from `rows` when the server has answered. */
  skillIds: string[];
  rows: TermsRow[];
  budget: SkillBudget | null;
  /** The cap on a percentage (95), for each slider's scale. */
  maxPercent: number;
  values: Record<string, string>;
  onChange: (id: string, value: string) => void;
  /** The refusal worded for a skill, null when there is none. */
  errorFor: (id: string) => string | null;
  /** The skill's name cell. */
  name: (id: string, row: TermsRow | undefined) => ReactNode;
  /** Title and Save / Cancel, kept at the top with the points left. */
  header?: ReactNode;
  stale?: boolean;
  testId?: string;
}) {
  const { t } = useTranslation();
  const focus = useAbilityFocus((s) => s.focus);
  const room = budget ? budget.available - typedTotal(values, skillIds) : 0;
  const shown: TermsRow[] = skillIds.map(
    (id) =>
      rows.find((r) => r.id === id) ?? {
        id,
        base: 0,
        points: 0,
        race: 0,
        dexterity: 0,
        armor: 0,
        rawPercent: 0,
        percent: 0,
      },
  );
  const known = new Set(rows.map((r) => r.id));
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {budget ? (
        <PoolBar
          header={header}
          left={budget.left}
          available={budget.available}
          leftOf={t("skills.leftOf", {
            count: budget.left,
            available: budget.available,
          })}
          terms={t("skills.budgetTerms", {
            formula: formula(
              t,
              budget.first,
              budget.perLevel,
              budget.level,
              budget.available,
            ),
            level: budget.level,
            spent: budget.spent,
          })}
          detail={
            budget.perSkillCap > 0 ? (
              <span
                className="min-w-0 text-label break-words text-ink-muted"
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
              </span>
            ) : undefined
          }
          stale={stale}
          testId="skill-points"
          className="bg-raised"
        >
          <span
            className="sr-only"
            data-over={budget.left < 0 ? "true" : undefined}
            data-testid="skill-points-over"
          >
            {budget.left < 0 ? t("skills.overBudget") : ""}
          </span>
        </PoolBar>
      ) : (
        header && <div className="sticky top-0 z-10 bg-raised">{header}</div>
      )}
      <div
        className={cn("flex flex-col gap-2", stale && "opacity-80")}
        data-stale={stale ? "true" : undefined}
        data-testid={testId}
      >
        <SkillTermsTable
          setId={setId}
          rows={shown}
          onRowFocus={(on) => focus(on ? "dex" : undefined)}
          name={(r) => name(r.id, known.has(r.id) ? r : undefined)}
          points={(r) => {
            const err = errorFor(r.id);
            const typed = Number(values[r.id]?.trim() ? values[r.id] : 0);
            const own = Number.isInteger(typed) && typed > 0 ? typed : 0;
            const limit = pointsLimit(r, budget?.perSkillCap ?? 0, maxPercent);
            return (
              <PointsSlider
                id={r.id}
                name={skillName(t, r.id)}
                value={values[r.id] ?? ""}
                limit={known.has(r.id) ? limit : maxPercent}
                reach={own + room}
                invalid={err !== null}
                describedBy={err ? `skill-error-${setId}-${r.id}` : undefined}
                onChange={(v) => onChange(r.id, v)}
              />
            );
          }}
        />
        {stale && (
          <p
            className="text-label break-words text-muted-foreground"
            data-testid="skill-preview-stale"
          >
            {t("skills.previewStale")}
          </p>
        )}
        <p className="text-label break-words text-ink-muted">
          {t("skills.sliderHint")}
        </p>
      </div>
    </div>
  );
}
