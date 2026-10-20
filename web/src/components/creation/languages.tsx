import { useTranslation } from "react-i18next";
import { Check, Lock, Warning } from "@phosphor-icons/react";
import { raceLanguages } from "@/lib/languages";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { cn } from "@/lib/utils";
import type { NonweaponPool } from "@/types/character";
import { Group } from "./kit";
import type { StepProps } from "./flow";

/**
 * The creation's languages (ADR 0032 step 4, ADR 0034 step 3.5 as amended by ADR 0037 step 18): on
 * the proficiency step, next to the non-weapon picks whose pool they spend (ADR 0037 step 6). The
 * catalogue as chips: the native language locked, the race's list bright, the others dimmed. The
 * pool's line is the draft's figures (`pool`), worded; an overspend is the server's
 * refusal at Créer, on `details.languages`.
 */
export function LanguagesGroup(props: StepProps & { pool?: NonweaponPool }) {
  const { options, errorFor, pool } = props;
  const { t } = useTranslation();
  if ((options.languages ?? []).length === 0) return null;
  const error = errorFor("details.languages");
  return (
    <Group
      title={t("details.fields.languages")}
      testId="language-chips"
      right={
        pool && (
          <span
            className={cn(
              "flex items-center gap-1 text-body",
              pool.spent > pool.available ? "text-warning" : "text-ink-muted",
            )}
            data-testid="languages-slots"
          >
            {pool.spent > pool.available && (
              <Warning aria-hidden weight="bold" className="size-4" />
            )}
            {t("creation.identity.languageSlots", {
              count: pool.languages,
              spent: pool.spent,
              available: pool.available,
            })}
          </span>
        )
      }
    >
      <LanguageChips {...props} />
      {error && (
        <p
          id="new-character-details.languages-error"
          role="alert"
          className="text-label text-destructive"
          data-testid="field-error"
        >
          {error}
        </p>
      )}
    </Group>
  );
}

/** The catalogue as chips: the native language locked, the race's list bright, the others dimmed. */
function LanguageChips({ options, draft, update }: StepProps) {
  const { t } = useTranslation();
  const name = useLanguageNames(options);
  const { native, list } = raceLanguages(options, draft.race);
  const catalogue = options.languages ?? [];
  const picked = draft.details.languages;
  const ordered = [
    ...catalogue.filter((l) => list.includes(l.code)),
    ...catalogue.filter((l) => !list.includes(l.code)),
  ].filter((l) => l.code !== native);
  const raceName = draft.race ? t(`details.races.${draft.race}`) : "";
  return (
    <>
      <div
        className="flex flex-wrap items-center gap-1.5"
        role="group"
        aria-label={t("details.fields.languages")}
      >
        {native && (
          <span
            className="flex min-h-7 items-center gap-1.5 rounded-md bg-raised px-2 py-1 text-body"
            title={t("details.languageNative")}
            data-testid="language-native"
          >
            <Lock aria-hidden className="size-3.5 shrink-0" />
            {name(native)}
            <span className="text-label text-ink-muted">
              {t("creation.identity.native")}
            </span>
          </span>
        )}
        {ordered.map((l) => {
          const on = picked.includes(l.code);
          const inList = list.includes(l.code);
          return (
            <button
              key={l.code}
              type="button"
              role="checkbox"
              aria-checked={on}
              onClick={() =>
                update({
                  details: {
                    languages: on
                      ? picked.filter((c) => c !== l.code)
                      : [...picked, l.code],
                  },
                })
              }
              className={cn(
                "flex min-h-7 items-center gap-1 rounded-md px-2 py-1 text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                on ? "chip-chosen" : "chip hover:bg-raised-hover",
                !inList && !on && "text-ink-muted",
              )}
            >
              {on && (
                <Check
                  aria-hidden
                  weight="bold"
                  className="size-3.5 shrink-0"
                />
              )}
              {name(l.code)}
            </button>
          );
        })}
      </div>
      {raceName && (
        <p className="text-label text-ink-muted">
          {t("creation.identity.languagesLead", {
            race: raceName,
          })}
        </p>
      )}
    </>
  );
}
