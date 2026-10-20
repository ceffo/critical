import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { skillName } from "@/journal/words";
import { cn } from "@/lib/utils";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { RogueSkill, SkillBudget } from "@/types/skills";
import { SkillBudgetSummary, SkillTerms } from "./SkillBreakdown";
import { classSkillsOf, previewDraft, typedPoints } from "./skillsPreview";

/**
 * A thief or a bard spends skill points after creation (ADR 0030 steps 17 and 18): the total
 * points of each skill, saved in one move the journal's Undo takes back. As the points change, the
 * server derives the percentages with their terms and the budget of the draft (a preview that
 * writes and logs nothing); the web only shows them. The server checks each total's range; the
 * budget stays a warning, never a refusal. A field the server refused shows its error under it.
 */
export function ThiefPointsEditor({
  skills,
  budget,
  onDone,
}: {
  skills: RogueSkill[];
  budget: SkillBudget | null;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const savePoints = useSkillsStore((s) => s.savePoints);
  const saving = useTableStore((s) => s.busy);
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(skills.map((s) => [s.id, String(s.points)])),
  );
  const [error, setError] = useState<{ field: string; text: string } | null>(
    null,
  );
  // What the server derives from the typed points; the stored ones until the first answer.
  const [shown, setShown] = useState({ skills, budget });
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      void previewDraft(values, skills).then((p) => {
        if (current && p)
          setShown({ skills: classSkillsOf(p), budget: p.budget });
      });
    }, 150);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [values, skills]);
  const maxPoints = skills[0]?.maxPoints ?? 0;
  const range = { min: 0, max: maxPoints };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const points = typedPoints(values, skills);
    if (!points) {
      const bad = skills.find(
        (s) => typedPoints({ [s.id]: values[s.id] ?? "" }, [s]) === null,
      );
      setError({
        field: `skillPoints.${bad?.id ?? ""}`,
        text: t("skills.pointsOutOfRange", range),
      });
      return;
    }
    const rejected = await savePoints(points);
    if (!rejected) {
      onDone();
      return;
    }
    setError({
      field: rejected.field ?? "skillPoints",
      text:
        rejected.message === "out_of_range"
          ? t("skills.pointsOutOfRange", range)
          : t("skills.pointsRefused"),
    });
  };

  const errorFor = (field: string) =>
    error?.field === field ? (
      <p role="alert" className="text-label break-words text-danger">
        {error.text}
      </p>
    ) : null;

  return (
    <form
      className="flex min-w-0 flex-col gap-3 px-2.5"
      aria-labelledby="thief-points-title"
      data-testid="thief-points-editor"
      onSubmit={(e) => void submit(e)}
    >
      <div className="flex flex-col gap-1">
        <h4 id="thief-points-title" className="text-body font-semibold">
          {t("skills.pointsTitle")}
        </h4>
        <p className="text-label break-words text-muted-foreground">
          {t("skills.pointsLead")}
        </p>
      </div>
      {shown.budget && (
        <SkillBudgetSummary budget={shown.budget} className="px-0" />
      )}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-3">
        {skills.map((s) => {
          const field = `skillPoints.${s.id}`;
          const name = skillName(t, s.id);
          const preview = shown.skills.find((p) => p.id === s.id);
          return (
            <div key={s.id} className="flex min-w-0 flex-col justify-end gap-1">
              <label
                htmlFor={`thief-points-${s.id}`}
                className={cn(labelClass, "break-words")}
              >
                {name}
              </label>
              <input
                id={`thief-points-${s.id}`}
                className={fieldClass}
                inputMode="numeric"
                aria-label={t("skills.pointsLabel", { name })}
                aria-invalid={error?.field === field || undefined}
                title={t("characters.range", range)}
                data-testid={`thief-points-${s.id}`}
                value={values[s.id] ?? ""}
                onChange={(e) => {
                  setValues((v) => ({ ...v, [s.id]: e.target.value }));
                  setError(null);
                }}
              />
              {errorFor(field)}
              {preview && (
                <SkillTerms skill={preview} cap={shown.budget?.perSkillCap} />
              )}
            </div>
          );
        })}
      </div>
      {errorFor("skillPoints")}
      <div className="flex flex-wrap gap-2">
        <Button
          type="submit"
          variant="main"
          size="sm"
          disabled={saving}
          data-testid="thief-points-save"
        >
          {t("skills.savePoints")}
        </Button>
        <Button
          type="button"
          size="sm"
          disabled={saving}
          onClick={onDone}
          data-testid="thief-points-cancel"
        >
          {t("skills.cancelPoints")}
        </Button>
      </div>
    </form>
  );
}
