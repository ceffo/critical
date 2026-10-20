import { useTranslation } from "react-i18next";
import { LanguagePicker } from "@/components/characters/LanguagePicker";
import type { CreationOptions } from "@/types/character";

/**
 * The languages pool of the Skills tab's editor (ADR 0037 step 18, ADR 0032 step 4): the catalogue
 * as the details card offered it (the race's PHB list first, its own language ticked and fixed),
 * the slots the languages beyond the native one and Common spend (`slots`, the server's preview,
 * ADR 0037 step 6) and the server's refusal on field `languages` (`error`, worded). Nothing without
 * a catalogue (before catalogue format 8).
 */
export function LanguagesPool({
  options,
  race,
  picked,
  slots,
  error,
  onChange,
}: {
  options: CreationOptions;
  race: string;
  picked: string[];
  slots: number;
  error: string | null;
  onChange: (codes: string[]) => void;
}) {
  const { t } = useTranslation();
  if ((options.languages ?? []).length === 0) return null;
  return (
    <section
      aria-labelledby="proficiency-editor-languages"
      className="flex min-w-0 flex-col gap-1.5"
      data-testid="proficiency-languages"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h5
          id="proficiency-editor-languages"
          className="text-body font-semibold"
        >
          {t("details.fields.languages")}
        </h5>
        {slots > 0 && (
          <span
            className="text-label text-muted-foreground tabular-nums"
            data-testid="proficiency-languages-slots"
          >
            {t("details.languageSlots", { count: slots })}
          </span>
        )}
      </div>
      <LanguagePicker
        idPrefix="proficiency-languages"
        options={options}
        race={race}
        picked={picked}
        onChange={onChange}
      />
      {error && (
        <p role="alert" className="text-label break-words text-danger">
          {error}
        </p>
      )}
    </section>
  );
}
