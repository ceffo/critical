import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/http";
import { fetchCreationOptions } from "@/api/characters";
import { previewProficiencies } from "@/api/proficiencies";
import { Button } from "@/components/ui/button";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { CreationOptions, ProficiencySlots } from "@/types/character";
import type { PickInput } from "./catalogue";
import { LanguagesPool } from "./LanguagesPool";
import { ProficiencyPicker } from "./ProficiencyPicker";
import { usePickRefusal, type Refusal } from "./refusal";

/**
 * The Skills tab's proficiency editor (ADR 0037 steps 12, 13 and 18): the character's languages
 * and non-weapon picks, the two spending the same pool, changed at any time (a level-up, catching
 * up, a correction) and saved in one move the journal's Undo takes back. As they change, the
 * server's preview answers the pools and each pick's figures (it writes and logs nothing); a
 * refusal (a pick under its cost, a save that grows an overspend, step 10, on the picks or the
 * languages) is shown with the server's words and marks the figures as out of date.
 */
export function ProficiencyEditor({
  cls,
  race,
  languages: stored,
  slots,
  onDone,
}: {
  cls: string;
  /** The character's race (its native language, its PHB list) and languages as last read. */
  race: string;
  languages: string[];
  /** The character's pools and picks as last read (`derived.proficiencySlots`). */
  slots: ProficiencySlots;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const save = useSkillsStore((s) => s.saveProficiencies);
  const saving = useTableStore((s) => s.busy);
  const [options, setOptions] = useState<CreationOptions | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [picks, setPicks] = useState<PickInput[]>(() =>
    slots.picks.map((p) => ({ id: p.id, slots: p.slots })),
  );
  const [languages, setLanguages] = useState<string[]>(() => [...stored]);
  const [figures, setFigures] = useState<ProficiencySlots>(slots);
  const [stale, setStale] = useState(false);
  const [previewError, setPreviewError] = useState<Refusal | null>(null);
  const [error, setError] = useState<Refusal | null>(null);
  const wording = usePickRefusal();

  useEffect(() => {
    let live = true;
    fetchCreationOptions().then(
      (o) => live && setOptions(o),
      () => live && setLoadError(true),
    );
    return () => {
      live = false;
    };
  }, []);

  // The live preview: the server's figures for the picks and languages shown, asked after a short
  // rest.
  const key = JSON.stringify({ nonweapon: picks, languages });
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      previewProficiencies(
        JSON.parse(key) as { nonweapon: PickInput[]; languages: string[] },
      ).then(
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
    const rejected = await save({ nonweapon: picks, languages });
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
      aria-labelledby="proficiency-editor-title"
      data-testid="proficiency-editor"
      onSubmit={(e) => void submit(e)}
    >
      {/* Save and Cancel beside the title: the catalogue below is long (ADR 0027 step 1). */}
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-1 basis-80 flex-col gap-1">
          <h4 id="proficiency-editor-title" className="text-body font-semibold">
            {t("proficiencies.title")}
          </h4>
          <p className="text-label break-words text-muted-foreground">
            {t("proficiencies.lead")}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button
            type="submit"
            variant="main"
            size="sm"
            disabled={saving || !options}
            title={t("proficiencies.saveHint")}
            data-testid="proficiency-save"
          >
            {t("proficiencies.save")}
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={saving}
            onClick={onDone}
            title={t("proficiencies.cancelHint")}
            data-testid="proficiency-cancel"
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
      {/* ADR 0037 steps 6 and 18: the languages beyond the native one and Common spend the pool the
          picks below spend; the slots they take are the server's preview. */}
      {options && (
        <LanguagesPool
          options={options}
          race={race}
          picked={languages}
          slots={figures.nonWeapon.languages}
          error={errorFor("languages")}
          onChange={(next) => {
            setLanguages(next);
            setError(null);
          }}
        />
      )}
      {options ? (
        <ProficiencyPicker
          options={options}
          cls={cls}
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
        shown.field !== "nonweapon" &&
        shown.field !== "languages" &&
        !picks.some((p) => `nonweapon.${p.id}` === shown.field) && (
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
    return {
      field: e.field ?? "nonweapon",
      code: e.code,
      message: e.message,
    };
  return {
    field: "nonweapon",
    message: e instanceof Error ? e.message : String(e),
  };
}
