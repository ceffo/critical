import { useTranslation } from "react-i18next";
import { ProficiencyPicker } from "@/components/proficiencies/ProficiencyPicker";
import { Note } from "./kit";
import type { StepProps } from "./flow";

/**
 * The creation's proficiency step (ADR 0037 step 14, ADR 0034): the editor of the Skills tab on the
 * draft. The slots and each pick's figures are the checks endpoint's answer for the draft's class,
 * level and Intelligence, asked again as the picks change; Créer sends the picks and the server
 * checks them as the editor's save does (nothing picked before: a draft over budget is refused).
 * Leftover slots never block: they are reminded.
 */
export function ProficienciesStep({
  options,
  draft,
  update,
  errorFor,
  checks,
}: StepProps) {
  const { t } = useTranslation();
  if (!options.proficiencies || options.proficiencies.length === 0)
    return <Note kind="info">{t("creation.proficiencies.none")}</Note>;
  return (
    <ProficiencyPicker
      options={options}
      cls={draft.cls}
      picks={draft.picks}
      onChange={(picks) => update({ picks })}
      figures={checks?.proficiencySlots ?? null}
      errorFor={errorFor}
    />
  );
}
