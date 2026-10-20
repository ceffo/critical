import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/http";
import { fetchCreationOptions } from "@/api/characters";
import { previewProficiencies } from "@/api/proficiencies";
import { Button } from "@/components/ui/button";
import { useCharacterStore } from "@/stores/character";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type {
  ProficiencySlots,
  WeaponPickInput,
  WeaponTypeOption,
} from "@/types/character";
import { usePickRefusal, type Refusal } from "./refusal";
import { WeaponPicker } from "./WeaponPicker";

/**
 * The Skills tab's weapon proficiency editor (ADR 0037 steps 7, 9, 12 and 13): the character's
 * weapon picks and, for a fighter, the specialization, changed at any time and saved in one move
 * the journal's Undo takes back. It sends the weapon pool only (`{weapon}`), so the non-weapon
 * picks stay as stored. As the picks change, the server's preview answers the pool (it writes and
 * logs nothing); a refusal is shown with the server's words and marks the figures as out of date.
 */
export function WeaponProficiencyEditor({
  cls,
  slots,
  onDone,
}: {
  cls: string;
  /** The character's pools and picks as last read (`derived.proficiencySlots`). */
  slots: ProficiencySlots;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const save = useSkillsStore((s) => s.saveProficiencies);
  const saving = useTableStore((s) => s.busy);
  const [types, setTypes] = useState<WeaponTypeOption[] | null>(null);
  // ADR 0030 step 12: the server says whether the class specializes.
  const specializes = useCharacterStore(
    (s) => s.character?.derived.specializes ?? false,
  );
  const [loadError, setLoadError] = useState(false);
  const [picks, setPicks] = useState<WeaponPickInput[]>(() =>
    (slots.weaponPicks ?? []).map((p) => ({ type: p.type, slots: p.slots })),
  );
  const [figures, setFigures] = useState<ProficiencySlots>(slots);
  const [stale, setStale] = useState(false);
  const [previewError, setPreviewError] = useState<Refusal | null>(null);
  const [error, setError] = useState<Refusal | null>(null);
  const wording = usePickRefusal();

  useEffect(() => {
    let live = true;
    fetchCreationOptions().then(
      (o) => {
        if (!live) return;
        setTypes(o.classes.find((c) => c.class === cls)?.weaponTypes ?? []);
      },
      () => live && setLoadError(true),
    );
    return () => {
      live = false;
    };
  }, [cls]);

  // The live preview: the server's figures for the picks shown, asked after a short rest.
  const key = JSON.stringify(picks);
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      previewProficiencies({
        weapon: JSON.parse(key) as WeaponPickInput[],
      }).then(
        (a) => {
          if (!current) return;
          if (a.proficiencySlots) setFigures(a.proficiencySlots);
          setStale(false);
          setPreviewError(null);
        },
        (e: unknown) => {
          if (!current) return;
          setStale(true);
          setPreviewError(refusalOf(e));
        },
      );
    }, 150);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [key]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const rejected = await save({ weapon: picks });
    if (!rejected) {
      onDone();
      return;
    }
    setError(refusalOf(rejected));
  };

  const shown = error ?? previewError;
  const errorFor = (field: string) =>
    shown && shown.field === field ? wording(shown) : null;

  return (
    <form
      className="flex min-w-0 flex-col gap-3 px-2.5"
      aria-labelledby="weapon-editor-title"
      data-testid="weapon-editor"
      onSubmit={(e) => void submit(e)}
    >
      {/* Save and Cancel beside the title, as the non-weapon editor (ADR 0027 step 1). */}
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-1 basis-80 flex-col gap-1">
          <h4 id="weapon-editor-title" className="text-body font-semibold">
            {t("weaponProficiencies.title")}
          </h4>
          <p className="text-label break-words text-muted-foreground">
            {t("weaponProficiencies.lead")}
            {specializes && ` ${t("weaponProficiencies.leadSpecializes")}`}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button
            type="submit"
            variant="main"
            size="sm"
            disabled={saving || !types}
            title={t("weaponProficiencies.saveHint")}
            data-testid="weapon-save"
          >
            {t("proficiencies.save")}
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={saving}
            onClick={onDone}
            title={t("proficiencies.cancelHint")}
            data-testid="weapon-cancel"
          >
            {t("proficiencies.cancel")}
          </Button>
        </div>
      </div>
      {loadError && (
        <p role="alert" className="text-body text-danger">
          {t("details.optionsError")}
        </p>
      )}
      {types ? (
        <WeaponPicker
          types={types}
          picks={picks}
          onChange={(next) => {
            setPicks(next);
            setError(null);
          }}
          figures={figures}
          stale={stale}
          errorFor={errorFor}
        />
      ) : (
        !loadError && (
          <p className="text-body text-muted-foreground">{t("loading")}</p>
        )
      )}
      {/* A refusal of a field the picker does not show (a pick no longer listed). */}
      {shown &&
        shown.field !== "weapon" &&
        !picks.some((p) => `weapon.${p.type}` === shown.field) &&
        !(types ?? []).some((o) => `weapon.${o.type}` === shown.field) && (
          <p role="alert" className="text-label break-words text-danger">
            {wording(shown)}
          </p>
        )}
    </form>
  );
}

/** A refusal of the server as the editor shows it: the field and code it named, its message. */
function refusalOf(e: unknown): Refusal {
  if (e instanceof ApiError)
    return { field: e.field ?? "weapon", code: e.code, message: e.message };
  return {
    field: "weapon",
    message: e instanceof Error ? e.message : String(e),
  };
}
