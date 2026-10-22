import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { OVER_BUDGET } from "@/api/http";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { skillName } from "@/journal/words";
import { cn } from "@/lib/utils";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { RogueSkill, SkillBudget } from "@/types/skills";
import { SkillBudgetSummary, SkillTerms } from "./SkillBreakdown";
import { previewDraft, skillSetOf, typedPoints } from "./skillsPreview";

/**
 * A thief or a bard spends skill points after creation (ADR 0030 steps 17 and 18): the total
 * points of each skill, saved in one move the journal's Undo takes back. As the points change, the
 * server derives the percentages with their terms and the budget of the draft (a preview that
 * writes and logs nothing); the web only shows them. The server checks each total's range and
 * the budget (ADR 0037 step 10: points that grow an overspend are refused, the save as the
 * preview; an overspend kept or reduced is a warning). A field the server refused shows its error
 * under it.
 */
export function ThiefPointsEditor({
  set,
  skills,
  budget,
  onDone,
}: {
  /** The skill set whose points are spent (ADR 0040 step 3.5). */
  set: string;
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
  // The last preview no longer matches the typed points (refused, failed or not a number), and the
  // field the server named when it refused them.
  const [stale, setStale] = useState(false);
  const [previewError, setPreviewError] = useState<{
    field: string;
    message?: string;
    code?: string;
  } | null>(null);
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      void previewDraft(set, values, skills).then((p) => {
        if (!current) return;
        if (p.ok) {
          const answered = skillSetOf(p.skills, set);
          setShown({
            skills: answered?.skills ?? [],
            budget: answered?.budget ?? null,
          });
          setStale(false);
          setPreviewError(null);
          return;
        }
        setStale(true);
        setPreviewError(
          p.field ? { field: p.field, message: p.message, code: p.code } : null,
        );
      });
    }, 150);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [set, values, skills]);
  const maxPoints = skills[0]?.maxPoints ?? 0;
  const range = { min: 0, max: maxPoints };
  // A refusal's wording: an over-budget one is the server code's, with its figures (the error's
  // message); a range or any other is the editor's own.
  const refusalText = (r: { message?: string; code?: string }) =>
    r.code === OVER_BUDGET && r.message
      ? r.message
      : r.message === "out_of_range"
        ? t("skills.pointsOutOfRange", range)
        : t("skills.pointsRefused");

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
    const rejected = await savePoints(set, points);
    if (!rejected) {
      onDone();
      return;
    }
    setError({
      field: rejected.field ?? "skillPoints",
      text: refusalText(rejected),
    });
  };

  const errorFor = (field: string) => {
    const text =
      error?.field === field
        ? error.text
        : previewError?.field === field
          ? refusalText(previewError)
          : null;
    return text ? (
      <p role="alert" className="text-label break-words text-danger">
        {text}
      </p>
    ) : null;
  };

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
        <div
          className={cn("flex flex-col gap-1", stale && "opacity-60")}
          data-stale={stale ? "true" : undefined}
          data-testid="skill-points-preview"
        >
          <SkillBudgetSummary budget={shown.budget} className="px-0" />
          {stale && (
            <p
              className="text-label break-words text-muted-foreground"
              data-testid="skill-preview-stale"
            >
              {t("skills.previewStale")}
            </p>
          )}
        </div>
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
                aria-invalid={
                  error?.field === field ||
                  previewError?.field === field ||
                  undefined
                }
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
