import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useCharacterStore } from "@/stores/character";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { ProficienciesInput, ProficiencySlots } from "@/types/character";
import { servedCatalogue } from "./catalogue";
import { LanguagesPool } from "./LanguagesPool";
import {
  offeredIds,
  refusalOf,
  schedulePreview,
  type PickPool,
} from "./pickEditor";
import { useAbilityScores } from "./pickHooks";
import { ProficiencyPicker } from "./ProficiencyPicker";
import { usePickRefusal, type Refusal } from "./refusal";
import { WeaponPicker } from "./WeaponPicker";

/** What differs between the pools: the i18n section and the test ids of the editor. */
const POOLS: Record<
  PickPool,
  {
    section: string;
    id: string;
    ids: { form: string; save: string; cancel: string };
  }
> = {
  nonWeapon: {
    section: "proficiencies",
    id: "proficiency-editor-title",
    ids: {
      form: "proficiency-editor",
      save: "proficiency-save",
      cancel: "proficiency-cancel",
    },
  },
  weapon: {
    section: "weaponProficiencies",
    id: "weapon-editor-title",
    ids: {
      form: "weapon-editor",
      save: "weapon-save",
      cancel: "weapon-cancel",
    },
  },
};

/** The picks the editor starts from, as stored: the pool's own and nothing of the other, so a save
 * leaves the other pool as stored. */
function startingPicks(
  pool: PickPool,
  slots: ProficiencySlots,
  languages: string[],
): ProficienciesInput {
  return pool === "weapon"
    ? {
        weapon: (slots.weaponPicks ?? []).map((p) => ({
          code: p.code,
          slots: p.slots,
        })),
      }
    : {
        nonweapon: slots.picks.map((p) => ({ id: p.id, slots: p.slots })),
        languages: [...languages],
      };
}

/**
 * The Skills activity's pick editor (ADR 0037 steps 7, 9, 12, 13 and 18), one for each pool of picks,
 * opened with the pool: the weapon picks and, for a fighter, the specialization; or the non-weapon
 * picks with the languages, the two spending the same pool. Changed at any time (a level-up,
 * catching up, a correction) and saved in one move the journal's Undo takes back, the pool's picks
 * only. As they change, the server's preview answers the pools and each pick's figures (it writes
 * and logs nothing); a refusal (a pick under its cost, a save that grows an overspend, step 10) is
 * shown with the server's words and marks the figures as out of date.
 */
export function PickEditor({
  pool,
  cls,
  race,
  languages: stored,
  slots,
  classes = "",
  onDone,
}: {
  pool: PickPool;
  cls: string;
  /** The character's race (its native language, its PHB list) and languages as last read. */
  race: string;
  languages: string[];
  /** The character's pools and picks as last read (`derived.proficiencySlots`). */
  slots: ProficiencySlots;
  /** The names of the character's classes, for the weapons they may not use. */
  classes?: string;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const cfg = POOLS[pool];
  const save = useSkillsStore((s) => s.saveProficiencies);
  const saving = useTableStore((s) => s.busy);
  const { options, failed: loadError } = useCreationOptions();
  const [draft, setDraft] = useState<ProficienciesInput>(() =>
    startingPicks(pool, slots, stored),
  );
  const [figures, setFigures] = useState<ProficiencySlots>(slots);
  // ADR 0037 step 22.4: what the server offers free, from the character as read, then each preview.
  const storedWarnings = useCharacterStore((s) => s.character?.warnings);
  const [offered, setOffered] = useState<string[]>(() =>
    offeredIds(storedWarnings),
  );
  const [stale, setStale] = useState(false);
  const [previewError, setPreviewError] = useState<Refusal | null>(null);
  const [error, setError] = useState<Refusal | null>(null);
  const wording = usePickRefusal();
  const scores = useAbilityScores();

  // The live preview of the picks shown, asked after a short rest.
  const key = JSON.stringify(draft);
  useEffect(
    () =>
      schedulePreview(JSON.parse(key) as ProficienciesInput, pool, {
        figures: (a, warnings) => {
          if (a) setFigures(a);
          setOffered(offeredIds(warnings));
          setStale(false);
          setPreviewError(null);
        },
        refused: (r) => {
          setStale(true);
          setPreviewError(r);
        },
      }),
    [key, pool],
  );

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const rejected = await save(draft);
    if (!rejected) {
      onDone();
      return;
    }
    setError(refusalOf(rejected, pool));
  };

  const change = (next: Partial<ProficienciesInput>) => {
    setDraft((d) => ({ ...d, ...next }));
    setError(null);
  };

  const shown = error ?? previewError;
  const errorFor = (field: string) =>
    shown && shown.field === field ? wording(shown) : null;
  // The server says which weapons the character may pick (all its classes judged together).
  const weapons = figures.weapons ?? slots.weapons ?? [];
  // A refusal of a field the picker does not show (a pick no longer listed).
  const hidden =
    shown &&
    (pool === "weapon"
      ? shown.field !== "weapon" &&
        !(draft.weapon ?? []).some((p) => `weapon.${p.code}` === shown.field) &&
        !weapons.some((o) => `weapon.${o.code}` === shown.field)
      : shown.field !== "nonweapon" &&
        shown.field !== "languages" &&
        !(draft.nonweapon ?? []).some(
          (p) => `nonweapon.${p.id}` === shown.field,
        ));

  const header = (
    // Save and Cancel with the title, kept at the top with the pool bar: the catalogue below is
    // long (ADR 0027 step 1).
    <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 px-2.5">
      <div className="flex min-w-0 flex-1 basis-60 flex-col gap-0.5">
        <h4 id={cfg.id} className="text-body font-semibold">
          {t(`${cfg.section}.title`)}
        </h4>
      </div>
      <div className="flex shrink-0 flex-wrap gap-2">
        <Button
          type="button"
          size="sm"
          disabled={saving}
          onClick={onDone}
          title={t("proficiencies.cancelHint")}
          data-testid={cfg.ids.cancel}
        >
          {t("proficiencies.cancel")}
        </Button>
        <Button
          type="submit"
          variant="main"
          size="sm"
          disabled={saving || (pool === "nonWeapon" && !options)}
          title={t(`${cfg.section}.saveHint`)}
          data-testid={cfg.ids.save}
        >
          {t("proficiencies.save")}
        </Button>
      </div>
    </div>
  );

  return (
    <form
      className="flex min-w-0 flex-col gap-3 px-2.5"
      aria-labelledby={cfg.id}
      data-testid={cfg.ids.form}
      onSubmit={(e) => void submit(e)}
    >
      {loadError && (
        <p role="alert" className="text-body text-danger">
          {t("details.optionsError")}
        </p>
      )}
      {options && pool === "nonWeapon" && (
        <ProficiencyPicker
          catalogue={servedCatalogue(options, cls, figures)}
          picks={draft.nonweapon ?? []}
          onChange={(nonweapon) => change({ nonweapon })}
          figures={figures}
          stale={stale}
          errorFor={errorFor}
          offered={offered}
          scores={scores}
          header={header}
          aside={
            // ADR 0037 steps 6 and 18: the languages beyond the native one and Common spend the
            // pool the picks spend; the slots they take are the server's preview.
            <LanguagesPool
              options={options}
              race={race}
              picked={draft.languages ?? []}
              slots={figures.nonWeapon.languages}
              error={errorFor("languages")}
              onChange={(languages) => change({ languages })}
            />
          }
        />
      )}
      {pool === "weapon" && (
        <WeaponPicker
          weapons={weapons}
          picks={draft.weapon ?? []}
          onChange={(weapon) => change({ weapon })}
          figures={figures}
          stale={stale}
          errorFor={errorFor}
          classes={classes}
          header={header}
        />
      )}
      {!options && !loadError && pool === "nonWeapon" && (
        <>
          {header}
          <p className="text-body text-muted-foreground">{t("app.loading")}</p>
        </>
      )}
      {hidden && shown && (
        <p role="alert" className="text-label break-words text-danger">
          {wording(shown)}
        </p>
      )}
    </form>
  );
}
