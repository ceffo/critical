import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  ALIGNMENT_CODES,
  SEX_CODES,
  TEXT_FIELDS,
  codeOrEmpty,
  hasAlignmentWarning,
  rangeWarnings,
} from "@/lib/characterDetails";
import { primaryEntry } from "@/lib/classEntries";
import { formatHeight, formatWeight, unitsFor } from "@/lib/measures";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useLanguageNames } from "@/lib/useLanguageNames";
import type { Character } from "@/types/character";

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[minmax(4.5rem,6rem)_minmax(0,1fr)] gap-2 text-body">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words tabular-nums">{children}</dd>
    </div>
  );
}

/**
 * The character's details as read-only rows, shared by the details popover and the card's lower
 * half: only what is set shows. The exception is a required value the character lacks (alignment,
 * sex), which is asked for in the danger colour, not hidden (ADR 0032 step 8.2). `withIdentity`
 * adds the name, class and level the popover opens with; the card has them in its header.
 * Everything shown is the server's: the language names, the slots the languages spend (ADR 0037
 * step 6) and the race's table ranges (ADR 0032 step 5.2, converted for display here).
 */
export function DetailRows({
  character: c,
  withIdentity = false,
}: {
  character: Character;
  withIdentity?: boolean;
}) {
  const { t, i18n } = useTranslation();
  // The language catalogue, for the names (ADR 0032 step 4): tolerant of a release that predates
  // it (its codes then show as they are).
  const catalogue = useCreationOptions();
  const languageName = useLanguageNames(catalogue.options);
  const languageSlots = c.derived.proficiencySlots?.nonWeapon.languages ?? 0;
  const primary = primaryEntry(c.classes)?.class ?? "";
  const classKey = `details.classes.${primary}`;
  const className = i18n.exists(classKey) ? t(classKey) : primary;
  const toPick = (
    <span className="text-danger" data-testid="details-missing">
      {t("details.missing")}
    </span>
  );
  // A value that is not a code (an old release's free text) reads as missing, never as a raw key.
  const alignment = codeOrEmpty(c.alignment, ALIGNMENT_CODES);
  const sex = codeOrEmpty(c.sex, SEX_CODES);
  const units = unitsFor(i18n.language);
  const outside = rangeWarnings(c);
  const rangeNote = (field: "height" | "weight" | "age") => {
    const w = outside[field];
    if (!w) return null;
    const show = (v: number) =>
      field === "height"
        ? formatHeight(v, units, i18n.language)
        : field === "weight"
          ? formatWeight(v, units, i18n.language)
          : String(v);
    return (
      <span
        className="block text-label text-muted-foreground"
        data-testid={`details-${field}-warning`}
      >
        {t(`details.warnings.${field}_outside_range`, {
          min: show(w.min),
          max: show(w.max),
        })}
      </span>
    );
  };
  return (
    <dl className="flex flex-col gap-2" data-testid="detail-rows">
      {withIdentity && (
        <>
          <Row label={t("characters.name")}>{c.name}</Row>
          <Row label={t("details.class")}>{className}</Row>
          <Row label={t("details.level")}>{c.derived.realizedLevel}</Row>
        </>
      )}
      {c.raceCode !== "" && (
        <Row label={t("details.raceCode")}>
          {t(`details.races.${c.raceCode}`)}
        </Row>
      )}
      <Row label={t("details.fields.alignment")}>
        {alignment === "" ? (
          toPick
        ) : (
          <>
            {t(`details.alignments.${alignment}`)}
            {hasAlignmentWarning(c) && (
              <span
                className="block text-label text-muted-foreground"
                data-testid="details-alignment-warning"
              >
                {t("details.warnings.alignment_not_allowed", { className })}
              </span>
            )}
          </>
        )}
      </Row>
      {c.age !== null && (
        <Row label={t("details.fields.age")}>
          {c.age}
          {rangeNote("age")}
        </Row>
      )}
      <Row label={t("details.fields.sex")}>
        {sex === "" ? toPick : t(`details.sexes.${sex}`)}
      </Row>
      {c.height_cm !== null && (
        <Row label={t("details.fields.height")}>
          {formatHeight(c.height_cm, units, i18n.language)}
          {rangeNote("height")}
        </Row>
      )}
      {c.weight_kg !== null && (
        <Row label={t("details.fields.weight")}>
          {formatWeight(c.weight_kg, units, i18n.language)}
          {rangeNote("weight")}
        </Row>
      )}
      {TEXT_FIELDS.filter((f) => c[f] !== "").map((f) => (
        <Row key={f} label={t(`details.fields.${f}`)}>
          {c[f]}
        </Row>
      ))}
      {c.languages.length > 0 && (
        <Row label={t("details.fields.languages")}>
          {!catalogue.options && !catalogue.failed
            ? "…"
            : c.languages.map(languageName).join(", ")}
          {languageSlots > 0 && (
            <span
              className="block text-label text-muted-foreground"
              data-testid="details-language-slots"
            >
              {t("details.languageSlots", { count: languageSlots })}
            </span>
          )}
        </Row>
      )}
    </dl>
  );
}
