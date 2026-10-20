import { useTranslation } from "react-i18next";
import { skillName } from "@/journal/words";
import {
  formatHeight,
  formatWeight,
  heightField,
  unitsFor,
  weightField,
} from "@/lib/measures";
import type { CharacterWarning, IdentityFigure } from "@/types/character";
import type { StepProps } from "./flow";

type Figure = IdentityFigure;
type T = ReturnType<typeof useTranslation>["t"];

/** A stored figure (cm, kg, years) in the language's units; `bare` leaves the unit out. */
export function useShow() {
  const { t, i18n } = useTranslation();
  const units = unitsFor(i18n.language);
  return (figure: Figure, v: number, bare = false): string =>
    figure === "height"
      ? bare
        ? heightField(v, units, i18n.language)
        : formatHeight(v, units, i18n.language)
      : figure === "weight"
        ? bare
          ? weightField(v, units, i18n.language)
          : formatWeight(v, units, i18n.language)
        : bare
          ? String(v)
          : `${v} ${t("details.roll.units.year")}`;
}

/** The words of a range warning (ADR 0032 step 5.2): the server's verdict and range. */
export function useWarningText() {
  const { t } = useTranslation();
  const show = useShow();
  return (w: CharacterWarning): string | null => {
    switch (w.code) {
      case "height_outside_range":
      case "weight_outside_range":
      case "age_outside_range": {
        if (w.min === undefined || w.max === undefined) return null;
        const f = w.field as Figure;
        return t(`details.warnings.${w.code}`, {
          min: show(f, w.min),
          max: show(f, w.max),
        });
      }
      default:
        return null;
    }
  };
}

/** The review's warnings, worded: the draft's, then the skill budget's (ADR 0030 step 17). */
export function useReviewWarnings(props: StepProps): string[] {
  const { t } = useTranslation();
  const warningText = useWarningText();
  const read = props.answer.current;
  if (!read?.derived) return [];
  const out: string[] = [];
  for (const w of read.warnings) {
    const text = warningText(w);
    if (text)
      out.push(
        t("creation.review.warningOf", {
          field: t(`details.fields.${w.field}`, { defaultValue: w.field }),
          text,
        }),
      );
  }
  for (const b of read.derived.skillBudget)
    out.push(
      t(`skills.budget.${b.code}`, {
        name: b.skill ? skillName(t, b.skill) : "",
        points: b.points,
        limit: b.limit,
      }),
    );
  return out;
}

/** The slot warnings of ADR 0037 step 11 (and a pick the catalogue lost), of any pool (`nonweapon`
 * or `weapon`): the proficiency editor shows its own pool from the server's figures, so the
 * identity summary leaves them all out. */
const SLOT_CODES = new Set([
  "slots_to_spend",
  "slots_over_budget",
  "proficiency_unknown",
]);

/** The fields the server warns about, for the character panel's summary ("1 avertissement : langues"). */
export function warnedFields(
  t: T,
  warnings: CharacterWarning[],
  budget: number,
): string[] {
  const out: string[] = [];
  for (const w of warnings) {
    if (SLOT_CODES.has(w.code)) continue;
    const f = t(`details.fields.${w.field}`, {
      defaultValue: w.field,
    }).toLocaleLowerCase();
    if (!out.includes(f)) out.push(f);
  }
  if (budget > 0) out.push(t("creation.review.skillsField"));
  return out;
}
