import { useTranslation } from "react-i18next";
import { OVER_BUDGET } from "@/api/http";

/** A refusal of the picks: the field the server named (`nonweapon.<id>`, `nonweapon`,
 * `weapon.<type>`, `weapon`), its error
 * code and its message (a field error's message is its code: `out_of_range`, `duplicate`…). */
export interface Refusal {
  field: string;
  code?: string;
  message?: string;
}

/**
 * The words of a picks refusal (ADR 0037 steps 10 and 12): an over-budget one is the server's
 * message, worded with its figures by `apiError.over_budget`; a pick's own refusal is its editor's
 * wording of its code (the non-weapon or the weapon one); anything else the server's text.
 */
export function usePickRefusal(): (r: Refusal) => string {
  const { t } = useTranslation();
  return (r) =>
    r.code === OVER_BUDGET
      ? (r.message ?? "")
      : r.field.startsWith("nonweapon") && r.message
        ? t(`proficiencies.errors.${r.message}`, {
            defaultValue: t("proficiencies.errors.refused"),
          })
        : r.field.startsWith("weapon") && r.message
          ? t(`weaponProficiencies.errors.${r.message}`, {
              defaultValue: t("weaponProficiencies.errors.refused"),
            })
          : (r.message ?? t("proficiencies.errors.refused"));
}
