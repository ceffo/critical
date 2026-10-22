import { useTranslation } from "react-i18next";
import { ProficiencyPicker } from "@/components/proficiencies/ProficiencyPicker";
import { WeaponPicker } from "@/components/proficiencies/WeaponPicker";
import { Note } from "./kit";
import { LanguagesGroup } from "./languages";
import type { StepProps } from "./flow";

/**
 * The creation's proficiency step (ADR 0037 step 14, ADR 0034): the weapon editor and the
 * non-weapon editor of the Skills tab on the
 * draft. The slots and each pick's figures are the draft's derived block (`POST
 * /api/characters/draft`), asked again as the picks change; Créer sends the picks and the server
 * checks them as the editor's save does (nothing picked before: a draft over budget is refused).
 * Leftover slots never block: they are reminded. The languages are picked here too, next to the
 * non-weapon picks whose pool they spend (ADR 0034 step 3.5 as amended by ADR 0037 step 18).
 */
export function ProficienciesStep(props: StepProps) {
  const { options, draft, update, errorFor, answer } = props;
  const { t } = useTranslation();
  const figures = answer.latest?.derived?.proficiencySlots ?? null;
  // The server's list for the draft (all its classes judged together, the weapon lift too).
  const types = figures?.weaponTypes;
  return (
    <div className="flex min-w-0 flex-col gap-6">
      {/* The weapons first: a short list, where the non-weapon catalogue is long. */}
      <section
        aria-labelledby="creation-weapons-title"
        className="flex min-w-0 flex-col gap-2"
      >
        <h2 id="creation-weapons-title" className="group-heading section-label">
          {t("weaponProficiencies.section")}
        </h2>
        {!types ? null : types.length > 0 ? (
          <WeaponPicker
            types={types}
            picks={draft.weaponPicks}
            onChange={(weaponPicks) => update({ weaponPicks })}
            figures={figures}
            errorFor={errorFor}
          />
        ) : (
          <Note kind="info">{t("creation.proficiencies.weaponsNone")}</Note>
        )}
      </section>
      <LanguagesGroup {...props} pool={figures?.nonWeapon} />
      <section
        aria-labelledby="creation-nonweapon-title"
        className="flex min-w-0 flex-col gap-2"
      >
        <h2
          id="creation-nonweapon-title"
          className="group-heading section-label"
        >
          {t("skills.proficiencies")}
        </h2>
        {!options.proficiencies || options.proficiencies.length === 0 ? (
          <Note kind="info">{t("creation.proficiencies.none")}</Note>
        ) : (
          <ProficiencyPicker
            options={options}
            cls={draft.cls}
            picks={draft.picks}
            onChange={(picks) => update({ picks })}
            figures={figures}
            errorFor={errorFor}
          />
        )}
      </section>
    </div>
  );
}
